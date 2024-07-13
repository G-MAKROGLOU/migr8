# INSTALLATION
``cd migr8``<br><br>
``go build .``

Add the executable to your path in order to execute it from anywhere

<h1 style="text-decoration:underline;">CONFIGURATION FILES & SAMPLES</h1>

<h2 style="text-decoration:underline;">INFRASTRUCTURE CONFIGURATION</h2>

```json
{
    "pat": "PERSONALL ACCESS TOKEN GENERATED FROM DEVOPS DASHBOARD",
    "devopsOrg": "https://dev.azure.com/<organization-name>",
    "agentPool": "name of agent pool",
    "infrastructure": [
        {   
            "type": "function",
            "name": "test-func-app",
            "storageAccount": "testfuncappsa",
            "resourceGroup": "Resource Group Name",
            "location": "westeurope",
            "runtime": "dotnet",
            "os": "windows",
            "pipeline": {
                "name": "TestFuncApp",
                "yamlPath": "./azure-pipelines.yml",
                "project": "functions",
                "repository": "FuncAppRepo",
                "branch": "main"
            },
            "settings": [
                {
                    "name": "EnvVarKey",
                    "value": "Value"
                }
            ]
        },
        {
            "type": "webapp",
            "name": "test-react-frontend",
            "resourceGroup": "Resource Group Name",
            "appServicePlan": "App Service Plan Name",
            "runtime": "NODE:16LTS",
            "location": "westeurope",
            "pipeline": {
                "name": "TestReactFrontEnd",
                "yamlPath": "./azure-pipelines.yml",
                "project": "test-react-frontend",
                "repository": "test-react-frontend",
                "branch": "main"
            },
            "settings": [
                {
                    "name": "REACT_APP_ENV_VAR",
                    "value": "Value"
                }
            ]
        },
        {
            "type": "webapp",
            "name": "test-nodejs-backend",
            "resourceGroup": "Resource Group Name",
            "appServicePlan": "App Service Plan Name",
            "runtime": "NODE:18LTS",
            "location": "westeurope",
            "pipeline": {
                "name": "TestNodeJsBackend",
                "yamlPath": "./azure-pipelines.yml",
                "project": "test-nodejs-backend",
                "repository": "test-nodejs-backend",
                "branch": "main"
            },
            "settings": [
                {
                    "name": "NODE_ENV",
                    "value": "production"
                }
            ]
        }
    ]
}



```
<h3 style="text-decoration:underline;">INFRASTRUCTURE CONFIGURATION PROPERTIES</h3>

```pat```       Personal Access Token created in Azure DevOPS

```devopsOrg``` The azure devops organization your projects belong to. Usually in the format https://dev.azure.com/<organization name\>

```agentPool``` The agent pool name that should be used for the pipelines

```infrastructure``` An array with the details of the applications to be created and deployed.


```infrastructure.type``` The type of the application. Currently ```webapp | function```

```infrastructure.name``` The name of the application. It will be used in the portal to identify the application, and also to create the domain of the application with ```.azurewebsites.net``` suffix. It has to be unique. If the name exists and the application belongs to the subscription, the creation is skipped.

```infrastructure.resourceGroup``` The resource group name under which the application will be created. It will be created if it doesn't exist.

```infrastructure.storageAccount``` Only for azure functions. A unique name for a storage account. It will be created if it doesn't exist.

```infrastructure.appServicePlan``` Only for azure webapps. A unique name for an app service plan. It will be created if it doesn't exist.

<h3 style="text-decoration:underline;">INFRASTRUCTURE INSTRUCTIONS AND REMARKS</h3>

<p>In order create any infrastructure (Function Apps & WebApps for now) you need to have installed:</p>

<ul>
    <li>az cli</li>
    <li>docker</li>
    <li>some az cli extensions</li>
</ul>

<h2 style="text-decoration:underline;">.NET 6.0 AZURE FUNCTIONS YAML TEMPLATE</h2>

```yml

trigger: none

parameters:
  - name: agentPool
    type: string
    default: 'ubuntu-latest'
  - name: agent
    type: string
    default: 'default'
  - name: azureSubscription
    type: string
    default: 'some placeholder value but don't validate yaml or a valid subscription. It can be overwritten by the parameters' 
  - name: appName
    type: string
    default: 'default'
  - name: resourceGroup
    type: string
    default: 'default'

variables:
  packagePath: '$(System.DefaultWorkingDirectory)/**/*.zip'
  _agentPool: ${{ parameters.agentPool }}
  _agent: ${{ parameters.agent }}
  _azSub: ${{ parameters.azureSubscription }}
  
pool:
    name: $(_agentPool)
    demands:
    - agent.name -equals $(_agent)

steps:
# Install the .NET Core SDK
- task: UseDotNet@2
  inputs:
    packageType: 'sdk'
    version: '6.x'  # Specify the .NET SDK version you need
    installationPath: $(Agent.ToolsDirectory)/dotnet

# Restore NuGet packages
- task: DotNetCoreCLI@2
  inputs:
    command: 'restore'
    projects: '**/*.csproj'

# Build the project
- task: DotNetCoreCLI@2
  inputs:
    command: 'build'
    projects: '**/*.csproj'
    arguments: '--configuration Release'

# Publish the project to a zip file
- task: DotNetCoreCLI@2
  inputs:
    command: 'publish'
    publishWebProjects: false
    projects: '**/*.csproj'
    arguments: '--configuration Release --output $(build.artifactStagingDirectory)'
    zipAfterPublish: true

# Deploy the Azure Function
- task: AzureFunctionApp@1
  displayName: 'Azure functions app deploy'
  inputs:
    azureSubscription: '$(_azSub)'
    appType: 'functionApp'
    appName: ${{ parameters.appName }}
    package: '$(build.artifactStagingDirectory)/*.zip'
    runtimeStack: 'DOTNET|6.0'  # Update according to your function's runtime stack

```


<h2 style="text-decoration:underline;">REACT YAML TEMPLATE</h2>

```yml
trigger: none

parameters:
  - name: agentPool
    type: string
    default: 'ubuntu-latest'
  - name: agent
    type: string
    default: 'default'
  - name: azureSubscription
    type: string
    default: 'some placeholder value but don't validate yaml or a valid subscription. It can be overwritten by the parameters'
  - name: appName
    type: string
    default: 'default'
  - name: REACT_APP_API_ENDPOINT
    type: string
    default: 'default'
#   ADD MORE VARIABLES AS NEEDED

variables:
  _agentPool: ${{ parameters.agentPool }}
  _agent: ${{ parameters.agent }}
  _azSub: ${{ parameters.azureSubscription }}

pool:
  name: $(_agentPool)
  demands:
  - agent.name -equals $(_agent)

steps:
- task: NodeTool@0
  inputs:
    versionSpec: '18.x'
  displayName: 'Install Node.js'

# ADJUST BUILD STEPS ACCORDING TO YOUR NEEDS. ADD LINTING ETC.
- script: |
    npm install --legacy-peer-deps
    npm run build
  displayName: 'npm install and build'
  env:
    REACT_APP_API_ENDPOINT: ${{ parameters.REACT_APP_API_ENDPOINT }}
    # REGISTER THE PARAMETERS DEFINED ABOVE

- task: ArchiveFiles@2
  inputs:
    # Folder where the React app build output is located, change according to your needs
    rootFolderOrFile: '$(System.DefaultWorkingDirectory)/build' 
    includeRootFolder: false
    archiveType: 'zip'
    archiveFile: '$(Build.ArtifactStagingDirectory)/$(Build.BuildId).zip'
    replaceExistingArchive: true
  displayName: 'Archive build output'

- task: AzureRmWebAppDeployment@4
  inputs:
    ConnectionType: 'AzureRM'
    azureSubscription: $(_azSub)
    appType: 'webApp'
    WebAppName: ${{ parameters.appName }}
    package: '$(Build.ArtifactStagingDirectory)/$(Build.BuildId).zip'
  displayName: 'Deploy to Azure App Service'
  
- publish: $(Build.ArtifactStagingDirectory)/$(Build.BuildId).zip
  artifact: drop

```

<h2 style="text-decoration:underline;">NODEJS YAML TEMPLATE WITH POST INSTALL SCRIPT</h2>

```yml

trigger: none

parameters:
  - name: agentPool
    type: string
    default: 'ubuntu-latest'
  - name: agent
    type: string
    default: 'default'
  - name: appName
    type: string
    default: 'default'
  - name: azureSubscription
    type: string
    default: 'some placeholder value but don't validate yaml or a valid subscription. It can be overwritten by the parameters'
  - name: NODE_ENV
    type: string
    default: 'production'
# ADD MORE VARIABLES AS NEEDED

variables:
  _azureSub: ${{ parameters.azureSubscription }}
  _agentPool: ${{ parameters.agentPool }}
  _agent: ${{ parameters.agent }}
  

stages:

- stage: DeployAndConfigure
  displayName: Deploy And Configure
  jobs:
  - job: Deploy
    displayName: Deploy And Configure
    pool:
      name: $(_agentPool)
      demands:
      - agent.name -equals $(_agent)

    steps:
    - task: NodeTool@0
      inputs:
        versionSource: 'spec'
        versionSpec: '12.x'
    
    - script: |
        npm install 
      displayName: 'Install node_modules'

    - task: ArchiveFiles@2
      displayName: 'Zip Source Code'
      inputs:
        rootFolderOrFile: '$(System.DefaultWorkingDirectory)'
        includeRootFolder: false
        archiveType: 'zip'
        archiveFile: '$(Build.ArtifactStagingDirectory)/$(Build.BuildId).zip'
        replaceExistingArchive: true

    - task: AzureRmWebAppDeployment@4
      displayName: 'Deploy, Set Env, and Move config folders' 
      inputs:
        ConnectionType: 'AzureRM'
        azureSubscription: '$(_azureSub)'
        appType: 'webApp'
        WebAppName: '${{ parameters.appName }}'
        package: '$(Build.ArtifactStagingDirectory)/$(Build.BuildId).zip'
        enableCustomDeployment: true
        DeploymentType: 'zipDeploy'
        RemoveAdditionalFilesFlag: true
        AppSettings: '-NODE_ENV production -CRON_API_URL ${{parameters.CRON_API_URL }} -DB_HOST ${{ parameters.DB_HOST }} -DB_PASS ${{ parameters.DB_PASS }} -DB_USER ${{ parameters.DB_USER }} -DEV_DB_HOST ${{ parameters.DEV_DB_HOST }} -DEV_DB_PASS ${{ parameters.DEV_DB_PASS }} -DEV_DB_USER ${{ parameters.DEV_DB_USER }} -NODE_HOST ${{ parameters.NODE_HOST }} -ROUTING_APIKEY ${{ parameters.ROUTING_APIKEY }}  -SWAGGER_STATS_PASSWORD ${{ parameters.SWAGGER_STATS_PASSWORD }} -SWAGGER_STATS_USERNAME ${{ parameters.SWAGGER_STATS_USERNAME }} -COMPANY_DB ${{ parameters.COMPANY_DB }} '
        # remove the lines below if you dont have post install needs
        ScriptType: 'File Path'
        ScriptPath: '$(System.DefaultWorkingDirectory)/post_install_script.bat' # post_install_script.bat should be part of your project in Azure Git

```

<hr/>

<h1 style="text-decoration:underline;">CLI MODES</h1>

<h2 style="text-decoration:underline;">Help</h2>

``migr8 -h`` - [Shows help information]


<hr/>

## Deploy Infrastructure

``migr8 infra`` with three available modes:

```complete``` Creates infrastructure and deploys based on the ```pipeline``` object

```create``` Only creates the infrastructure. The ```pipeline``` object can be omitted

```deploy``` Only deploys based on the ```pipeline``` object

### Flags

``-i`` The absolute path to an infrastructure configuration file. See example on top


### Examples


#### Create and deploy infrastructure

```migr8 infra complete -i C:\Users\test-stack.json```


#### Create infrastructure

```migr8 infra create -i C:\Users\test-stack.json```


#### Deploy source code to infrastructure

```migr8 infra deploy -i C:\Users\test-stack.json```


<hr>

## Create a Service Connection
devops org -> project -> project settings -> service connections -> new service connection with details:
- azure resource manager (next)
- service principal (automatic) (next)
- Subscription
- Select Subscription from dropdown
- Leave Resource Group Empty
- Service Connection Name = Subscription Id (this convention is important for now)
- Grant access permission to all pipelines

<hr>

## Create a Self Hosted Agent
devops org -> project -> project settings -> agent pools -> add pool with details:
- New 
- SelfHosted
- Agent Pool Name (save it to be used in the infra config)
- Grant Access To All Pipelines

### REMARKS
- For free parallelization a project needs to be turned public. (as soon as jobs are finished, you can turn it private again. az cli does not allow management  of project visibility)
- Ιf you don't turn the project public, jobs will be queued normally but sequentially regardless of the amount of agents that you spawned. Turning a project public will allow you to run multiple pipelines in parallel, allowing for faster deployments of any number of applications.
- Self Hosted agents need to be created manually
- Service connections need to be created manually and with a specific convention (at least for now)
