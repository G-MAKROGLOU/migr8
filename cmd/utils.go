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

// ReadJSON reads a JSON file and deserializes it into the provided model pointer.
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

// NewResourceGroupCreate creates a new ResourceGroupCreate struct.
func NewResourceGroupCreate(app AppDetails) *azresourcegroup.ResourceGroupCreate {
	return &azresourcegroup.ResourceGroupCreate{
		Name:     app.ResourceGroup,
		Location: app.Location,
	}
}

// NewStorageAccountCreate creates a new StorageAccountCreate struct.
func NewStorageAccountCreate(funcApp AppDetails) *azstorageaccount.StorageAccountCreate {
	return &azstorageaccount.StorageAccountCreate{
		Name:          funcApp.StorageAccount,
		Location:      funcApp.Location,
		ResourceGroup: funcApp.ResourceGroup,
	}
}

// NewFunctionCreate creates a new CreateFunction struct.
func NewFunctionCreate(funcApp AppDetails) *azfunction.CreateFunction {
	// Bug fix: previously used make([]Setting, len) which pre-fills with N zero-value
	// items, then appended N real items — producing 2N entries with the first half blank.
	// Use make([]Setting, 0, len) to allocate capacity without creating empty elements.
	settings := make([]azfunction.Setting, 0, len(funcApp.Settings))
	for _, s := range funcApp.Settings {
		settings = append(settings, azfunction.Setting{
			Name:  s.Name,
			Value: s.Value,
		})
	}

	return &azfunction.CreateFunction{
		Name:           funcApp.Name,
		StorageAccount: funcApp.StorageAccount,
		Location:       funcApp.Location,
		ResourceGroup:  funcApp.ResourceGroup,
		Os:             funcApp.Os,
		Runtime:        funcApp.Runtime,
		Settings:       settings,
	}
}

// NewWebAppCreate creates a new WebAppCreate struct.
func NewWebAppCreate(webApp AppDetails) *azwebapp.WebAppCreate {
	return &azwebapp.WebAppCreate{
		Name:           webApp.Name,
		ResourceGroup:  webApp.ResourceGroup,
		AppServicePlan: webApp.AppServicePlan,
		Runtime:        webApp.Runtime,
	}
}

// NewAppServicePlanCreate creates a new AppServicePlanCreate struct.
func NewAppServicePlanCreate(webApp AppDetails) *azappservice.AppServicePlanCreate {
	// Bug fix: Name was never set, causing 'az appservice plan create --name ""'.
	// Bug fix: Location was set twice; removed the duplicate assignment.
	return &azappservice.AppServicePlanCreate{
		Name:          webApp.AppServicePlan,
		ResourceGroup: webApp.ResourceGroup,
		Location:      webApp.Location,
	}
}

// NewPipelineCreate creates a new PipelineCreate struct.
func NewPipelineCreate(appDetails AppDetails, devopsOrg string) *azpipelines.PipelineCreate {
	return &azpipelines.PipelineCreate{
		Name:       appDetails.Pipeline.Name,
		DevOPSOrg:  devopsOrg,
		Project:    appDetails.Pipeline.Project,
		YamlPath:   appDetails.Pipeline.YamlPath,
		Repository: appDetails.Pipeline.Repository,
		Branch:     appDetails.Pipeline.Branch,
	}
}
