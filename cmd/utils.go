package cmd

import (
	"encoding/json"
	"os"

	"github.com/G-MAKROGLOU/infrastructure/azappservice"
	"github.com/G-MAKROGLOU/infrastructure/azfunction"
	"github.com/G-MAKROGLOU/infrastructure/azpipelines"
	"github.com/G-MAKROGLOU/infrastructure/azresourcegroup"
	"github.com/G-MAKROGLOU/infrastructure/azstorageaccount"
	"github.com/G-MAKROGLOU/infrastructure/azwebapp"
	"github.com/fatih/color"
)

// ReadJSON reads a json file and deserializes it into K
func ReadJSON[K interface{}](path string, model K) error {
	config, readErr := os.ReadFile(path)
	if readErr != nil {
		color.Red("[ERR:] => READ FILE => %s", readErr.Error())
		return readErr
	}

	configErr := json.Unmarshal(config, &model)

	if configErr != nil {
		color.Red("[ERR:] => JSON UNMARSHAL => %s", configErr.Error())
		return configErr
	}
	return nil
}

// NewResourceGroupCreate creates a new ResourceGroupCreate struct
func NewResourceGroupCreate(funcApp AppDetails) *azresourcegroup.ResourceGroupCreate {
	resGroupDetails := new(azresourcegroup.ResourceGroupCreate)
	resGroupDetails.Name = funcApp.ResourceGroup
	resGroupDetails.Location = funcApp.Location
	return resGroupDetails
}

// NewStorageAccountCreate creates a new StorageAccountCreate struct
func NewStorageAccountCreate(funcApp AppDetails) *azstorageaccount.StorageAccountCreate {
	saDetails := new(azstorageaccount.StorageAccountCreate)
	saDetails.Name = funcApp.StorageAccount
	saDetails.Location = funcApp.Location
	saDetails.ResourceGroup = funcApp.ResourceGroup
	return saDetails
}

// NewFunctionCreate creates a new FunctionCreate struct
func NewFunctionCreate(funcApp AppDetails) *azfunction.CreateFunction {
	funcAppDetails := new(azfunction.CreateFunction)
	funcAppDetails.Name           = funcApp.Name
	funcAppDetails.StorageAccount = funcApp.StorageAccount
	funcAppDetails.Location       = funcApp.Location
	funcAppDetails.ResourceGroup  = funcApp.ResourceGroup
	funcAppDetails.Os             = funcApp.Os
	funcAppDetails.Runtime        = funcApp.Runtime
	funcAppDetails.Settings       = make([]azfunction.Setting, len(funcApp.Settings))

	for _, setting := range funcApp.Settings {
		funcSetting := new(azfunction.Setting)
		funcSetting.Name = setting.Name
		funcSetting.Value = setting.Value

		funcAppDetails.Settings = append(funcAppDetails.Settings, *funcSetting)
	}

	return funcAppDetails
}

// NewWebAppCreate creates a new WebAppCreate struct
func NewWebAppCreate(webApp AppDetails) *azwebapp.WebAppCreate {
	waDetails := new(azwebapp.WebAppCreate)
	waDetails.Name           = webApp.Name
    waDetails.ResourceGroup  = webApp.ResourceGroup
    waDetails.AppServicePlan = webApp.AppServicePlan
    waDetails.Runtime        = webApp.Runtime
	return waDetails
}

// NewAppServicePlanCreate creates a new AppServicePlanCreate struct
func NewAppServicePlanCreate(webApp AppDetails) *azappservice.AppServicePlanCreate {
	aspDetails := new(azappservice.AppServicePlanCreate)
	aspDetails.Location = webApp.Location
	aspDetails.ResourceGroup = webApp.ResourceGroup
	aspDetails.Location = webApp.Location
	return aspDetails
}

// NewPipelineCreate creates a new PipelineCreate struct
func NewPipelineCreate(appDetails AppDetails, devopsOrg string) *azpipelines.PipelineCreate {
	pipelineDetails := new(azpipelines.PipelineCreate)

	pipelineDetails.Name       = appDetails.Pipeline.Name
    pipelineDetails.DevOPSOrg  = devopsOrg
    pipelineDetails.Project    = appDetails.Pipeline.Project
    pipelineDetails.YamlPath   = appDetails.Pipeline.YamlPath
    pipelineDetails.Repository = appDetails.Pipeline.Repository
    pipelineDetails.Branch     = appDetails.Pipeline.Branch

	return pipelineDetails
}
