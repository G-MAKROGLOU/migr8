package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/G-MAKROGLOU/containers"
	"github.com/G-MAKROGLOU/devops/agentpool"
	"github.com/G-MAKROGLOU/infrastructure/azappservice"
	"github.com/G-MAKROGLOU/infrastructure/azfunction"
	"github.com/G-MAKROGLOU/infrastructure/azlogin"
	"github.com/G-MAKROGLOU/infrastructure/azpipelines"
	"github.com/G-MAKROGLOU/infrastructure/azresourcegroup"
	"github.com/G-MAKROGLOU/infrastructure/azstorageaccount"
	"github.com/G-MAKROGLOU/infrastructure/azwebapp"
	"github.com/fatih/color"
	prettyTable "github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

var (
	agentsRes    = []ChannelRes{}
	infraRes     = []ChannelRes{}
	pipelinesRes = []ChannelRes{}
	queuesRes    = []ChannelRes{}
	infraCmd     = &cobra.Command{
		Use:               "infra",
		Short:             "Create all the infrastructure needed by an application stack",
		Long:              "Create all the infrastructure needed by an application stack",
		PersistentPreRun:  prerun,
		PersistentPostRun: cleanup,
		Version: rootCmd.Version,
	}
	onlyInfraCmd = &cobra.Command{
		Use:   "create",
		Short: "Create all the infrastructure needed by an application stack",
		Long:  "Create all the infrastructure needed by an application stack",
		Run:   run,
		Version: rootCmd.Version,
	}
	onlyDeployCmd = &cobra.Command{
		Use:   "deploy",
		Short: "Deploy all applications based on existing azure pipelines",
		Long:  "Deploy all applications based on existing azure pipelines",
		Run:   run,
		Version: rootCmd.Version,
	}
	fullCmd = &cobra.Command{
		Use:   "complete",
		Short: "Create and deploy all the infrastructure needed by an application stack",
		Long:  "Create and deploy all the infrastructure needed by an application stack",
		Run:   run,
		Version: rootCmd.Version,
	}
)

// opeational
func init() {
	infraCmd.PersistentFlags().StringVarP(&infraConfigPath, "infraConfig", "i", "", "The infrastructre configuration to be deployed")
	infraCmd.MarkFlagRequired("infraConfig")

	infraCmd.AddCommand(onlyInfraCmd)
	infraCmd.AddCommand(onlyDeployCmd)
	infraCmd.AddCommand(fullCmd)

	rootCmd.AddCommand(infraCmd)
}

func prerun(cmd *cobra.Command, args []string) {
	configErr := ReadJSON(infraConfigPath, &infraConfig)
	if configErr != nil {
		os.Exit(1)
	}
	validateConfig()
	login()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		color.Yellow("RECEIVED TERMINATION SIGNAL. CLEANING UP RESOURCES...")
		cleanup(nil, nil)
		os.Exit(0)
	}()
}

func run(cmd *cobra.Command, args []string) {
	isCompleteRun := cmd.CalledAs() == "complete"
	isDeployOnly := cmd.CalledAs() == "deploy"
	isCreateOnly := cmd.CalledAs() == "create"

	agentsChan := make(chan ChannelRes, len(infraConfig.Infrastructure))
	infraChan := make(chan ChannelRes, len(infraConfig.Infrastructure))
	pipelineChan := make(chan ChannelRes, len(infraConfig.Infrastructure))
	queuesChan := make(chan ChannelRes, len(infraConfig.Infrastructure))

	if isCompleteRun || isDeployOnly {
		copyBuildContextConfig()
		initalizeDockerClient()
		buildAgentPoolImage()
		startAgents(agentsChan)
		for agent := range agentsChan {
			agentsRes = append(agentsRes, agent)
		}
	}

	if isCompleteRun || isCreateOnly {
		createInfrastructure(infraChan)
		for infra := range infraChan {
			infraRes = append(infraRes, infra)
		}
	}

	if isCompleteRun || isDeployOnly {
		createPipelines(isCompleteRun, pipelineChan)
		for pipeline := range pipelineChan {
			pipelinesRes = append(pipelinesRes, pipeline)
		}

		quequePipelines(queuesChan)
		for queue := range queuesChan {
			queuesRes = append(queuesRes, queue)
		}
	}
}

func cleanup(cmd *cobra.Command, args []string) {

	isCompleteRun := cmd.CalledAs() == "complete"
	isCreateRun := cmd.CalledAs() == "create"
	isDeployRun := cmd.CalledAs() == "deploy"

	var waitGroup sync.WaitGroup
	waitGroup.Add(1)

	if isCompleteRun || isDeployRun {
		waitGroup.Add(1)

		go func() {
			defer waitGroup.Done()
			// stop and remove all created containers
			for _, contID := range agentpool.ContainerIDs {
				stopErr := containers.StopContainer(contID)
				if stopErr != nil {
					color.Red(stopErr.Error())
				}
				purgeErr := containers.PurgeContainer(contID)
				if purgeErr != nil {
					color.Red(purgeErr.Error())
				}
			}

			// remove image
			imgExists, delImgErr := containers.DeleteImage("azp_agent:latest")
			if delImgErr != nil {
				color.Red(delImgErr.Error())
			}
			if !imgExists {
				color.Yellow("[WARN:] IMAGE DOES NOT EXIST. SKIPPING IMAGE DELETION")
			}

			// remove any possible dangling images
			pruneReport, pruneErr := containers.PruneDanglingImages()
			if pruneErr != nil {
				color.Red(pruneErr.Error())
			}
			if pruneErr == nil {
				color.Cyan("[INFO:] PRUNED %d DANGLING IMAGES. RECLAIMED %d SPACE", len(pruneReport.ImagesDeleted), pruneReport.SpaceReclaimed)
			}
		}()
	}

	go func() {
		defer waitGroup.Done()
		// delete img build ctx
		delErr := os.RemoveAll("migr8_agentpool_build_ctx")
		if delErr != nil {
			color.Red("[ERR]: FAILED TO DELETE BUILD CONTEXT DIRECTORY")
		}
	}()

	waitGroup.Wait()

	if cmd != nil {
		// produce results table
		printResults(isCompleteRun, isCreateRun, isDeployRun)
	}
}

// core run functions
func validateConfig() {
	if strings.TrimSpace(infraConfig.Pat) == "" {
		color.Yellow("[WARN:] NO PERSONAL ACCESS TOKEN FOUND. SKIPPING ANY RESOURCE ALLOCATIONS")
		os.Exit(1)
	}
	if strings.TrimSpace(infraConfig.DevOpsOrg) == "" {
		color.Yellow("[WARN:] NO AZURE DEVOPS ORGANIZATION URL FOUND. SKIPPING ANY RESOURCE ALLOCATIONS")
		os.Exit(1)
	}
	if len(infraConfig.Infrastructure) == 0 || infraConfig.Infrastructure == nil {
		color.Yellow("[WARN:] NO INFRASTRUCTURE DESCRIPTION FOUND. SKIPPING ANY RESOURCE ALLOCATIONS")
		os.Exit(1)
	}
}

func login() {
	loginErr := azlogin.AzureLogin()
	if loginErr != nil {
		color.Red("[ERR:] => AZ LOGIN => %s", loginErr.Error())
		os.Exit(1)
	}
	azlogin.SelectSubscription()
}

func copyBuildContextConfig() {
	err := agentpool.CreateBuildCtx("migr8_agentpool_build_ctx")
	if err != nil {
		color.Red(err.Error())
		os.Exit(1)
	}
}

func initalizeDockerClient() {
	color.Cyan("[INFO:] INITIALIZING DOCKER CLIENT")
	initDockerClientErr := containers.InitializeDockerClient()
	if initDockerClientErr != nil {
		color.Red(initDockerClientErr.Error())
		os.Exit(1)
	}
}

func buildAgentPoolImage() {
	dir, homeDirErr := os.UserHomeDir()
	if homeDirErr != nil {
		color.Red("[ERR:] => HOME DIR => %s", homeDirErr.Error())
		os.Exit(1)
	}

	color.Cyan("[INFO:] BUILDING AGENT POOL IMAGE")

	buildErr := containers.BuildImage(dir+"\\migr8_agentpool_build_ctx", "azp_agent")
	if buildErr != nil {
		color.Red("[ERR:] => IMAGE BUILD => %s", buildErr.Error())
		os.Exit(1)
	}

	color.Cyan("[INFO:] AGENT POOL IMAGE BUILT SUCCESSFULLY")
}

func startAgents(agentsChan chan<- ChannelRes) {
	color.Cyan("[INFO:] STARTING ALL AGENTS")

	var waitGroup sync.WaitGroup
	for _, appDetails := range infraConfig.Infrastructure {
		waitGroup.Add(1)
		go agentWorker(appDetails, &waitGroup, agentsChan)
	}
	waitGroup.Wait()
	close(agentsChan)
}

func createInfrastructure(infraChan chan<- ChannelRes) {
	color.Cyan("[INFO:] CREATING ALL INFRASTRUCTURE")

	var waitGroup sync.WaitGroup
	for _, appDetails := range infraConfig.Infrastructure {
		waitGroup.Add(1)
		go infraWorker(appDetails, &waitGroup, infraChan)
	}
	waitGroup.Wait()
	close(infraChan)
}

func createPipelines(isCompleteRun bool, pipelineChan chan<- ChannelRes) {
	color.Cyan("[INFO:] CREATING ALL PIPELINES")

	var waitGroup sync.WaitGroup

	for _, appDetails := range infraConfig.Infrastructure {
		waitGroup.Add(1)
		go pipelineWorker(isCompleteRun, appDetails, &waitGroup, pipelineChan)
	}
	waitGroup.Wait()
	close(pipelineChan)
}

func quequePipelines(queuesChan chan<- ChannelRes) {
	color.Cyan("[INFO:] QUEUEING ALL PIPELINES")

	var waitGroup sync.WaitGroup

	for _, appDetails := range infraConfig.Infrastructure {
		waitGroup.Add(1)
		go queuePipelineWorker(appDetails, &waitGroup, queuesChan)
	}
	waitGroup.Wait()
	close(queuesChan)
}

// workers
func agentWorker(appDetails AppDetails, waitGroup *sync.WaitGroup, agentsChan chan<- ChannelRes) {
	defer waitGroup.Done()

	containerName := appDetails.Name + "_deployment_agent"
	configDetails := agentpool.ConfigDetails{
		Org:           infraConfig.DevOpsOrg,
		Pat:           infraConfig.Pat,
		Pool:          infraConfig.AgentPool,
		ContainerName: containerName,
	}

	channelRes := ChannelRes{
		Key:   appDetails.Name,
		Value: true,
	}
	_, agentPoolErr := agentpool.StartAgentPool(configDetails)
	if agentPoolErr != nil {
		color.Red("[ERR:] => WEBAPPS => %s", agentPoolErr.Error())
		channelRes.Value = false
	}
	agentsChan <- channelRes
}

func infraWorker(appDetails AppDetails, waitGroup *sync.WaitGroup, infraChan chan<- ChannelRes) {
	defer waitGroup.Done()
	channelRes := ChannelRes{
		Key:   appDetails.Name,
		Value: true,
	}

	if appDetails.Type == "function" {
		err := createFuncApp(appDetails)
		if err != nil {
			color.Red(err.Error())
			channelRes.Value = false
		}
	}

	if appDetails.Type == "webapp" {
		err := createWebapp(appDetails)
		if err != nil {
			color.Red(err.Error())
			channelRes.Value = false
		}
	}
	infraChan <- channelRes
}

func pipelineWorker(isCompleteRun bool, appDetails AppDetails, waitGroup *sync.WaitGroup, pipelineChan chan<- ChannelRes) {
	defer waitGroup.Done()
	channelRes := ChannelRes{
		Key:   appDetails.Name,
		Value: true,
	}

	isInfraCreated := isResourceCreated(infraRes, appDetails.Name)

	if isCompleteRun && !isInfraCreated {
		color.Yellow("[PIPELINE %s:] [WARN:] THE INFRASTRUCTURE WAS NOT CREATED. SKIPPING PIPELINE CREATION FOR UNKNOWN INFRASTRUCTURE", appDetails.Pipeline.Name)
		channelRes.Value = false
	}

	if (isCompleteRun && isInfraCreated) || !isCompleteRun {
		pipelineDetails := NewPipelineCreate(appDetails, infraConfig.DevOpsOrg)

		err := azpipelines.CreatePipelineFromYaml(*pipelineDetails)
		if err != nil {
			color.Red("[PIPELINE %s:] [ERR:] => [AZ PIPELINES] => FAILED TO CREATE PIPELINE FOR APP %s OF TYPE %s => %s", appDetails.Name, appDetails.Type, err.Error())
			channelRes.Value = false
		}
	}

	pipelineChan <- channelRes
}

func queuePipelineWorker(appDetails AppDetails, waitGroup *sync.WaitGroup, queuesChan chan<- ChannelRes) {
	defer waitGroup.Done()

	channelRes := ChannelRes{
		Key:   appDetails.Name,
		Value: true,
	}

	isAgentUp := isResourceCreated(agentsRes, appDetails.Name)
	isPipelineUp := isResourceCreated(pipelinesRes, appDetails.Name)
	areAgentAndPipelineUp := isAgentUp && isPipelineUp

	if !areAgentAndPipelineUp {
		if !isAgentUp {
			color.Yellow("[WARN:] => [PIPELINE %s] => THE AGENT WAS NOT CREATED. SKIPPING PIPELINE QUEUEING FOR OFFLINE AGENT", appDetails.Pipeline.Name)
		}
		if !isPipelineUp {
			color.Yellow("[WARN:] => [PIPELINE %s] => THE PIPELINE WAS NOT CREATED. SKIPPING PIPELINE QUEUEING FOR UNKNOWN PIPELINE", appDetails.Pipeline.Name)
		}
		channelRes.Value = false
	}

	if areAgentAndPipelineUp {
		parameters := getPipelineParams(appDetails)

		pipelineDetails := NewPipelineCreate(appDetails, infraConfig.DevOpsOrg)

		pipelineQueueRes, err := azpipelines.QueuePipeline(*pipelineDetails, parameters)
		if err != nil {
			color.Red("[ERR:]=> [AZ PIPELINES %s] => FAILED TO RUN PIPELINE => %s", appDetails.Pipeline.Name, err.Error())
			channelRes.Value = false
		}

		// start pipeline polling only if there was no error queueing the pipeline
		var pipelineStatus azpipelines.PipelineStatus
		if channelRes.Value {
			color.Cyan("[PIPELINE %s] STARTING PIPELINE STATUS POLLING", appDetails.Pipeline.Name)

			for {
				pipeline, err := azpipelines.GetPipelineStatus(infraConfig.DevOpsOrg, appDetails.Pipeline.Project, pipelineQueueRes.ID)
				if err == nil && pipeline.Status == "completed" {
					pipelineStatus = pipeline
					break
				}
				if err == nil && pipeline.Status != "completed" {
					color.Yellow("[PIPELINE %s:] [STATUS: %s] WAITING FOR PIPELINE TO FINISH.", appDetails.Pipeline.Name, pipeline.Status)
				}
				if err != nil {
					color.Red(err.Error())
					channelRes.Value = false
					break
				}
				time.Sleep(30 * time.Second)
			}
		}

		// if its still true, it means the pipeline completed, check status for proper logging
		if channelRes.Value {
			if pipelineStatus.Result == "failed" {
				color.Red("[ERR:] => [PIPELINE %s] COMPLETED WITY STATUS %s. CHECK THE DEVOPS PORTAL FOR THE ERRORS AND RERUN WITH 'migr8 infra deploy'", appDetails.Pipeline.Name, pipelineStatus.Result)
			}
			if pipelineStatus.Result == "succeeded" {
				color.Green("[PIPELINE %s:] COMPLETED WITH STATUS %s.", appDetails.Pipeline.Name, pipelineStatus.Result)
			}
		}
	}
	queuesChan <- channelRes
}

// infrastructure wrappers
func createFuncApp(funcApp AppDetails) error {

	color.Cyan("[FUNCAPP %s:] CREATING AZURE FUNCTION APP", funcApp.Name)
	errorMsg := ""

	resGroupDetails := NewResourceGroupCreate(funcApp)

	rgError := azresourcegroup.CreateAzureResourceGroup(*resGroupDetails)
	if rgError != nil {
		errorMsg = fmt.Sprintf("[ERR:] [FUNCAPP %s:] => [AZURE RESOURCE GROUP] => %s", funcApp.Name, rgError.Error())
		return errors.New(errorMsg)
	}

	// make sure that the storage account does not exist. This is important to avoid overwriting function logs etc.
	storageAccDetails := NewStorageAccountCreate(funcApp)

	saError := azstorageaccount.CreateAzureStorageAccount(*storageAccDetails)
	if saError != nil {
		errorMsg = fmt.Sprintf("[ERR:] [FUNCAPP %s:] => [AZURE STORAGE ACCOUNT] => %s", funcApp.Name, saError.Error())
		return errors.New(errorMsg)
	}

	// make sure the functionapp does not exist. This is important to avoid overwriting function during deployment
	funcAppDetails := NewFunctionCreate(funcApp)

	faError := azfunction.CreateAzureFunction(*funcAppDetails)
	if faError != nil {
		errorMsg = fmt.Sprintf("[ERR:] [FUNCAPP %s:] => [AZURE FUNCTIONAPP] => %s", funcApp.Name, faError.Error())
		return errors.New(errorMsg)
	}

	// if the functionapp has not environment variables just print a message
	if len(funcApp.Settings) == 0 {
		color.Yellow("[WARN:] [FUNCAPP %s:] AZURE FUNCTIONAPP SETTINGS | NO SETTINGS TO UPDATE. SKIPPING SETTINGS CONFIGURATION", funcApp.Name)
	}

	// set the environment variables for the functionapp
	if len(funcApp.Settings) != 0 {
		faSettingsErr := azfunction.SetAzureFunctionEnv(*funcAppDetails)
		if faSettingsErr != nil {
			errorMsg = fmt.Sprintf("[ERR:] [FUNCAPP %s:] => [AZURE FUNCTIONAPP SETTINGS] => %s", funcApp.Name, faSettingsErr.Error())
			return errors.New(errorMsg)
		}
	}

	return nil
}

func createWebapp(webapp AppDetails) error {

	color.Cyan("[WEBAPP %s:] CREATING AZURE WEBAPP", webapp.Name)
	errorMsg := ""

	resGroupDetails := NewResourceGroupCreate(webapp)

	// make sure the resource group for the webapp exists
	rgError := azresourcegroup.CreateAzureResourceGroup(*resGroupDetails)
	if rgError != nil {
		errorMsg = fmt.Sprintf("[ERR:] [WEBAPP %s:] => [AZURE RESOURCE GROUP] => %s", webapp.Name, rgError.Error())
		return errors.New(errorMsg)
	}

	aspDetails := NewAppServicePlanCreate(webapp)

	// make sure the app service plan exists
	aseError := azappservice.CreateAzureAppServicePlan(*aspDetails)
	if aseError != nil {
		errorMsg = fmt.Sprintf("[ERR:] [WEBAPP %s:] => [AZURE APP SERVICE PLAN] => %s", webapp.Name, aseError.Error())
		return errors.New(errorMsg)
	}

	webappDetails := NewWebAppCreate(webapp)

	// create the webapp
	waError := azwebapp.CreateAzureWebApp(*webappDetails)
	if waError != nil {
		errorMsg = fmt.Sprintf("[ERR:] [WEBAPP %s:] => [AZURE WEBAPP] => %s", webapp.Name, waError.Error())
		return errors.New(errorMsg)
	}

	return nil
}

// utility functions
func isResourceCreated(channelResults []ChannelRes, key string) bool {
	if len(channelResults) == 0 {
		return false
	}

	isCreated := true
	for _, res := range channelResults {
		if res.Key == key {
			isCreated = res.Value
			break
		}
	}
	return isCreated
}

func printResults(isCompleteRun bool, isCreateRun bool, isDeployRun bool) {
	t := prettyTable.NewWriter()
	t.SetOutputMirror(os.Stdout)

	color.Cyan("\n############### MIGR8 RESULTS ##############\n")

	t.AppendHeader(prettyTable.Row{"APP NAME", "AGENT", "INFRASTRUCTURE", "PIPELINE", "QUEUE"})

	for _, app := range infraConfig.Infrastructure {
		appName := app.Name
		agent := "N/A"
		infra := "N/A"
		pipeline := "N/A"
		queue := "N/A"

		if len(agentsRes) > 0 {
			agentCreated := isResourceCreated(agentsRes, appName)
			if (isCompleteRun && agentCreated) || (isDeployRun && agentCreated) {
				agent = "SUCCESS"
			}
			if (isCompleteRun && !agentCreated) || (isDeployRun && !agentCreated) {
				agent = "FAILED"
			}
		}

		if len(infraRes) > 0 {
			infrastructureCreated := isResourceCreated(infraRes, appName)
			if (isCompleteRun && infrastructureCreated) || (isCreateRun && infrastructureCreated) {
				infra = "SUCCESS"
			}
			if (isCompleteRun && !infrastructureCreated) || (isCreateRun && !infrastructureCreated) {
				infra = "FAILED"
			}
		}

		if len(pipelinesRes) > 0 {
			pipelineCreated := isResourceCreated(pipelinesRes, appName)
			if (isCompleteRun && pipelineCreated) || (isDeployRun && pipelineCreated) {
				pipeline = "SUCCESS"
			}
			if (isCompleteRun && !pipelineCreated) || (isDeployRun && !pipelineCreated) {
				pipeline = "FAILED"
			}
		}

		if len(queuesRes) > 0 {
			queueCreated := isResourceCreated(queuesRes, appName)
			if (isCompleteRun && queueCreated) || (isDeployRun && queueCreated) {
				queue = "SUCCESS"
			}
			if (isCompleteRun && !queueCreated) || (isDeployRun && !queueCreated) {
				queue = "FAILED"
			}
		}

		t.AppendRow(prettyTable.Row{
			appName, agent, infra, pipeline, queue,
		})
		t.AppendSeparator()
	}

	t.Render()
}

func getPipelineParams(appDetails AppDetails) []string {
	parameters := []string{
		"azureSubscription=" + azlogin.SelectedSubscription.ID,
		"appName=" + appDetails.Name,
		"agentPool=" + infraConfig.AgentPool,
		"agent=" + appDetails.Name + "_deployment_agent",
	}

	if appDetails.Type == "function" {
		parameters = append(parameters, "resourceGroup="+appDetails.ResourceGroup)
	}

	if appDetails.Type == "webapp" {
		for _, env := range appDetails.Settings {
			specialChars, _ := regexp.Compile(`[!@#\$%\^&\*\(\)_\+\=\[\]\{\};'"\\|,<>?~]`)
			param := env.Name + "=" + env.Value
			if specialChars.MatchString(env.Value) {
				param = fmt.Sprintf("%s=\"%s\"", env.Name, env.Value)
			}
			parameters = append(parameters, param)
		}
	}

	return parameters
}
