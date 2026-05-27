package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
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
		Version:           rootCmd.Version,
	}
	onlyInfraCmd = &cobra.Command{
		Use:     "create",
		Short:   "Create all the infrastructure needed by an application stack",
		Long:    "Create all the infrastructure needed by an application stack",
		Run:     run,
		Version: rootCmd.Version,
	}
	onlyDeployCmd = &cobra.Command{
		Use:     "deploy",
		Short:   "Deploy all applications based on existing azure pipelines",
		Long:    "Deploy all applications based on existing azure pipelines",
		Run:     run,
		Version: rootCmd.Version,
	}
	fullCmd = &cobra.Command{
		Use:     "complete",
		Short:   "Create and deploy all the infrastructure needed by an application stack",
		Long:    "Create and deploy all the infrastructure needed by an application stack",
		Run:     run,
		Version: rootCmd.Version,
	}
)

func init() {
	infraCmd.PersistentFlags().StringVarP(&infraConfigPath, "infraConfig", "i", "", "The infrastructure configuration to be deployed")
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
		// Bug fix: pass nil so cleanup() knows it was called from the signal handler.
		// The nil guard at the top of cleanup() handles this safely.
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

		// Bug fix: renamed from quequePipelines (typo)
		queuePipelines(queuesChan)
		for queue := range queuesChan {
			queuesRes = append(queuesRes, queue)
		}
	}
}

func cleanup(cmd *cobra.Command, args []string) {
	// Bug fix: cmd.CalledAs() panics on a nil receiver. This function is called both
	// by Cobra (valid cmd) and directly from the signal handler (nil cmd). Determine
	// the run mode only when cmd is non-nil; default to false so the container/image
	// cleanup still runs based on whether containers were actually created.
	isCompleteRun := false
	isCreateRun := false
	isDeployRun := false

	if cmd != nil {
		isCompleteRun = cmd.CalledAs() == "complete"
		isCreateRun = cmd.CalledAs() == "create"
		isDeployRun = cmd.CalledAs() == "deploy"
	}

	var waitGroup sync.WaitGroup
	waitGroup.Add(1)

	// Clean up containers whenever there are any — this covers both the normal
	// post-run path and the signal-handler path where cmd is nil but containers
	// may already have been created.
	if isCompleteRun || isDeployRun || len(agentpool.ContainerIDs) > 0 {
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
		// delete build context directory
		if err := os.RemoveAll("migr8_agentpool_build_ctx"); err != nil {
			color.Red("[ERR]: FAILED TO DELETE BUILD CONTEXT DIRECTORY")
		}
	}()

	waitGroup.Wait()

	if cmd != nil {
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
	ctxPath := "migr8_agentpool_build_ctx"

	stat, _ := os.Stat(ctxPath)
	if stat == nil {
		if err := agentpool.CreateBuildCtx(ctxPath); err != nil {
			color.Red(err.Error())
			os.Exit(1)
		}
	}
}

func initalizeDockerClient() {
	color.Cyan("[INFO:] INITIALIZING DOCKER CLIENT")
	if err := containers.InitializeDockerClient(); err != nil {
		color.Red(err.Error())
		os.Exit(1)
	}
}

func buildAgentPoolImage() {
	dir, homeDirErr := os.Getwd()
	if homeDirErr != nil {
		color.Red("[ERR:] => HOME DIR => %s", homeDirErr.Error())
		os.Exit(1)
	}

	color.Cyan("[INFO:] BUILDING AGENT POOL IMAGE")

	// Bug fix: use filepath.Join instead of hardcoded "\\" (Windows-only separator).
	buildErr := containers.BuildImage(filepath.Join(dir, "migr8_agentpool_build_ctx"), "azp_agent")
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

// queuePipelines queues all deployment pipelines concurrently.
// Bug fix: renamed from quequePipelines (typo).
func queuePipelines(queuesChan chan<- ChannelRes) {
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

	channelRes := ChannelRes{Key: appDetails.Name, Value: true}

	if _, err := agentpool.StartAgentPool(configDetails); err != nil {
		color.Red("[ERR:] => AGENT => %s", err.Error())
		channelRes.Value = false
	}
	agentsChan <- channelRes
}

func infraWorker(appDetails AppDetails, waitGroup *sync.WaitGroup, infraChan chan<- ChannelRes) {
	defer waitGroup.Done()
	channelRes := ChannelRes{Key: appDetails.Name, Value: true}

	var err error
	switch appDetails.Type {
	case "function":
		err = createFuncApp(appDetails)
	case "webapp":
		err = createWebapp(appDetails)
	}

	if err != nil {
		color.Red(err.Error())
		channelRes.Value = false
	}
	infraChan <- channelRes
}

func pipelineWorker(isCompleteRun bool, appDetails AppDetails, waitGroup *sync.WaitGroup, pipelineChan chan<- ChannelRes) {
	defer waitGroup.Done()
	channelRes := ChannelRes{Key: appDetails.Name, Value: true}

	isInfraCreated := isResourceCreated(infraRes, appDetails.Name)

	if isCompleteRun && !isInfraCreated {
		color.Yellow("[PIPELINE %s:] [WARN:] THE INFRASTRUCTURE WAS NOT CREATED. SKIPPING PIPELINE CREATION FOR UNKNOWN INFRASTRUCTURE", appDetails.Pipeline.Name)
		channelRes.Value = false
		pipelineChan <- channelRes
		return
	}

	pipelineDetails := NewPipelineCreate(appDetails, infraConfig.DevOpsOrg)
	if err := azpipelines.CreatePipelineFromYaml(*pipelineDetails); err != nil {
		color.Red("[PIPELINE %s:] [ERR:] => [AZ PIPELINES] => FAILED TO CREATE PIPELINE FOR APP %s OF TYPE %s => %s", appDetails.Pipeline.Name, appDetails.Name, appDetails.Type, err.Error())
		channelRes.Value = false
	}

	pipelineChan <- channelRes
}

func queuePipelineWorker(appDetails AppDetails, waitGroup *sync.WaitGroup, queuesChan chan<- ChannelRes) {
	defer waitGroup.Done()

	channelRes := ChannelRes{Key: appDetails.Name, Value: true}

	isAgentUp := isResourceCreated(agentsRes, appDetails.Name)
	isPipelineUp := isResourceCreated(pipelinesRes, appDetails.Name)

	if !isAgentUp {
		color.Yellow("[WARN:] => [PIPELINE %s] => THE AGENT WAS NOT CREATED. SKIPPING PIPELINE QUEUEING FOR OFFLINE AGENT", appDetails.Pipeline.Name)
		channelRes.Value = false
	}
	if !isPipelineUp {
		color.Yellow("[WARN:] => [PIPELINE %s] => THE PIPELINE WAS NOT CREATED. SKIPPING PIPELINE QUEUEING FOR UNKNOWN PIPELINE", appDetails.Pipeline.Name)
		channelRes.Value = false
	}

	if !channelRes.Value {
		queuesChan <- channelRes
		return
	}

	parameters := getPipelineParams(appDetails)
	pipelineDetails := NewPipelineCreate(appDetails, infraConfig.DevOpsOrg)

	pipelineQueueRes, err := azpipelines.QueuePipeline(*pipelineDetails, parameters)
	if err != nil {
		color.Red("[ERR:]=> [AZ PIPELINES %s] => FAILED TO RUN PIPELINE => %s", appDetails.Pipeline.Name, err.Error())
		channelRes.Value = false
		queuesChan <- channelRes
		return
	}

	color.Cyan("[PIPELINE %s] STARTING PIPELINE STATUS POLLING", appDetails.Pipeline.Name)

	var pipelineStatus azpipelines.PipelineStatus
	for {
		pipeline, err := azpipelines.GetPipelineStatus(infraConfig.DevOpsOrg, appDetails.Pipeline.Project, pipelineQueueRes.ID)
		if err != nil {
			color.Red(err.Error())
			channelRes.Value = false
			break
		}
		if pipeline.Status == "completed" {
			pipelineStatus = pipeline
			break
		}
		color.Yellow("[PIPELINE %s:] [STATUS: %s] WAITING FOR PIPELINE TO FINISH.", appDetails.Pipeline.Name, pipeline.Status)
		time.Sleep(30 * time.Second)
	}

	if channelRes.Value {
		switch pipelineStatus.Result {
		case "failed":
			// Bug fix: typo "WITY" → "WITH"
			color.Red("[ERR:] => [PIPELINE %s] COMPLETED WITH STATUS %s. CHECK THE DEVOPS PORTAL FOR THE ERRORS AND RERUN WITH 'migr8 infra deploy'", appDetails.Pipeline.Name, pipelineStatus.Result)
		case "succeeded":
			color.Green("[PIPELINE %s:] COMPLETED WITH STATUS %s.", appDetails.Pipeline.Name, pipelineStatus.Result)
		}
	}

	queuesChan <- channelRes
}

// infrastructure wrappers

func createFuncApp(funcApp AppDetails) error {
	color.Cyan("[FUNCAPP %s:] CREATING AZURE FUNCTION APP", funcApp.Name)

	if err := azresourcegroup.CreateAzureResourceGroup(*NewResourceGroupCreate(funcApp)); err != nil {
		return fmt.Errorf("[ERR:] [FUNCAPP %s:] => [AZURE RESOURCE GROUP] => %s", funcApp.Name, err.Error())
	}

	if err := azstorageaccount.CreateAzureStorageAccount(*NewStorageAccountCreate(funcApp)); err != nil {
		return fmt.Errorf("[ERR:] [FUNCAPP %s:] => [AZURE STORAGE ACCOUNT] => %s", funcApp.Name, err.Error())
	}

	funcAppDetails := NewFunctionCreate(funcApp)

	if err := azfunction.CreateAzureFunction(*funcAppDetails); err != nil {
		return fmt.Errorf("[ERR:] [FUNCAPP %s:] => [AZURE FUNCTIONAPP] => %s", funcApp.Name, err.Error())
	}

	if len(funcApp.Settings) == 0 {
		color.Yellow("[WARN:] [FUNCAPP %s:] AZURE FUNCTIONAPP SETTINGS | NO SETTINGS TO UPDATE. SKIPPING SETTINGS CONFIGURATION", funcApp.Name)
		return nil
	}

	if err := azfunction.SetAzureFunctionEnv(*funcAppDetails); err != nil {
		return fmt.Errorf("[ERR:] [FUNCAPP %s:] => [AZURE FUNCTIONAPP SETTINGS] => %s", funcApp.Name, err.Error())
	}

	return nil
}

func createWebapp(webapp AppDetails) error {
	color.Cyan("[WEBAPP %s:] CREATING AZURE WEBAPP", webapp.Name)

	if err := azresourcegroup.CreateAzureResourceGroup(*NewResourceGroupCreate(webapp)); err != nil {
		return fmt.Errorf("[ERR:] [WEBAPP %s:] => [AZURE RESOURCE GROUP] => %s", webapp.Name, err.Error())
	}

	if err := azappservice.CreateAzureAppServicePlan(*NewAppServicePlanCreate(webapp)); err != nil {
		return fmt.Errorf("[ERR:] [WEBAPP %s:] => [AZURE APP SERVICE PLAN] => %s", webapp.Name, err.Error())
	}

	if err := azwebapp.CreateAzureWebApp(*NewWebAppCreate(webapp)); err != nil {
		return fmt.Errorf("[ERR:] [WEBAPP %s:] => [AZURE WEBAPP] => %s", webapp.Name, err.Error())
	}

	return nil
}

// utility functions

// isResourceCreated reports whether the given key was recorded as successfully
// created. Returns false both when the key was recorded as failed AND when the
// key is not found in the results at all (safe default).
func isResourceCreated(channelResults []ChannelRes, key string) bool {
	for _, res := range channelResults {
		if res.Key == key {
			return res.Value
		}
	}
	// Bug fix: previously returned true by default when the key was not found.
	return false
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
			if (isCompleteRun || isDeployRun) && agentCreated {
				agent = "SUCCESS"
			}
			if (isCompleteRun || isDeployRun) && !agentCreated {
				agent = "FAILED"
			}
		}

		if len(infraRes) > 0 {
			infrastructureCreated := isResourceCreated(infraRes, appName)
			if (isCompleteRun || isCreateRun) && infrastructureCreated {
				infra = "SUCCESS"
			}
			if (isCompleteRun || isCreateRun) && !infrastructureCreated {
				infra = "FAILED"
			}
		}

		if len(pipelinesRes) > 0 {
			pipelineCreated := isResourceCreated(pipelinesRes, appName)
			if (isCompleteRun || isDeployRun) && pipelineCreated {
				pipeline = "SUCCESS"
			}
			if (isCompleteRun || isDeployRun) && !pipelineCreated {
				pipeline = "FAILED"
			}
		}

		if len(queuesRes) > 0 {
			queueCreated := isResourceCreated(queuesRes, appName)
			if (isCompleteRun || isDeployRun) && queueCreated {
				queue = "SUCCESS"
			}
			if (isCompleteRun || isDeployRun) && !queueCreated {
				queue = "FAILED"
			}
		}

		t.AppendRow(prettyTable.Row{appName, agent, infra, pipeline, queue})
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
			// Bug fix: removed conditional shell-quoting. exec.Command passes args
			// directly to the OS without a shell — adding literal '"' chars around
			// values via fmt.Sprintf corrupted the setting values. Pass name=value
			// verbatim; special characters (/, \, =, etc.) are handled correctly.
			parameters = append(parameters, env.Name+"="+env.Value)
		}
	}

	return parameters
}

