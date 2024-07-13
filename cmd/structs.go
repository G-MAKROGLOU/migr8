package cmd


type (
	// InfraConfig ~ the JSON representation of the infrastructure to be created and deployed
	InfraConfig struct {
		App            string       `json:"app"`
		Pat            string       `json:"pat"`
		DevOpsOrg      string       `json:"devopsOrg"`
		Infrastructure []AppDetails `json:"infrastructure"`
		AgentPool      string       `json:"agentPool"`
	}

	// AppDetails ~ the general details of the app to be created
	AppDetails struct {
		Type           string        `json:"type"`
		Name           string        `json:"name"`
		StorageAccount string        `json:"storageAccount"`
		ResourceGroup  string        `json:"resourceGroup"`
		Location       string        `json:"location"`
		Pipeline       Pipeline      `json:"pipeline"`
		Settings       []AppSettings `json:"settings"`
		AppServicePlan string        `json:"appServicePlan"`
		Runtime        string        `json:"runtime"`
		Os             string        `json:"os"`
	}

	// Pipeline ~ the details of the deployment pipeline
	Pipeline struct {
		Name           string `json:"name"`
		YamlPath       string `json:"yamlPath"`
		Project        string `json:"project"`
		Repository     string `json:"repository"`
		Branch         string `json:"branch"`
		ServiceAccount string `json:"serviceAccount"`
	}

	// AppSettings ~ the environment variables for the given webapp or function app
	AppSettings struct {
		Name        string `json:"name"`
		SlotSetting bool   `json:"slotSetting"`
		Value       string `json:"value"`
	}

	// ChannelRes ~ a generic response that all channels can send
	ChannelRes struct {
		Key   string
		Value bool
	}
)
