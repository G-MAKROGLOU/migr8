# WHY

<p>
    By default, Azure provided the ability to run deployment pipelines on their own infrastructure for free but this functionality now comes with a cost after a series of events that were using the pipelines for malicious purposes.
    Currently, if you wish to use Azure's infrastructure to run your pipelines you have two options:    
</p>

<ul>
    <li>Purchase compute</li>
    <li>Request free compute for a private project that has to pass a review first</li>
</ul>

<p>
    A second option is to run Self Hosted Agent Pools on your own infrastructure, and purchase parallelization with the burdens of purchasing and maintaining the hardware. Also, in case you need parellelization, you'll have to
    purchase it.
</p>

<p>
    The third option that Azure provides is to run Self Hosted Agent Pools in containers. This is the option migr8 takes advantage of to allow free n* amount of parallel deployment jobs completely for free without a dedicated host
    or setup overhead. Just be sure that your machine can afford spawning the amount of agent containers that you are trying to spawn.
</p>

# HOW IT WORKS

<p>
    Simply put, the only thing you'll have to do is to describe your infrastructure and how you want it to be deployed with JSON and YML files. migr8 then will take care of creating the infrastructure, spawning an agent 
    container for each infrastructure piece described, creating the pipeline from the yml description in your repository, and finally queueing the pipeline for deployment. You can also opt-in to specific functionalities and for example, 
    just create the infrastructure, or just deploy it. 
</p>

<p>The steps required sum up to:</p>

<ol>
    <li>Create an azure-pipelines.yml file in your project with your pipeline description and trigger: none. (See examples below)</li>
    <li>Push your code to Azure DevOPS</li>
    <li>Turn your project public. This is important for free parallelization when queueing pipelines but not required (See reasons below). It is not required when you are just creating infrastructure.</li>
    <li>Describe your infrastructure as shown in the examples below</li>
    <li>Run migr8 in your preferred mode.</li>
    <li>Turn your project private again if your turned it public in step 3.</li>
</ol>

<p>When creating infrastructure, migr8 first looks if all the required resources exist. If they already exist, it skips the creation. The same applies when creating pipelines.</p>

<p>See further down below for configuration reference, example yml descriptions, usage examples, and more.</p>

# INSTALLATION

<h2 style="text-decoration:underline;">MANUALLY</h2>

``clone the repo``
``cd migr8``<br><br>
``go build .``
Add the executable to your path in order to execute it from anywhere

<h2 style="text-decoration:underline;">With go cli</h2>

```go install github.com/G-MAKROGLOU/migr8@latest```

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

```infrastructure.runtime``` A runtime name. WebApps have different runtime naming convetions from Azure Functions. You can run ```az functionapp list-runtimes``` and ```az webapp list-runtimes``` to get the list of runtimes for Azure Functions and WebApps respsectively.

```infrastructure.location``` A location name according to the Azure location naming conventions. You can run ```az account list-locations``` to get the name of the location you want your service to be created.

```infrastructure.pipeline.name``` The name of an existing (or not) pipeline. If the pipeline does not exist, it will be created.

```infrastructure.pipeline.yamlPath``` The path to the azure-pipelines.yml file. Use ```./azure-pipelines.yaml``` if it's located at the root of the repository.

```infrastructure.pipeline.project``` The project inside the Azure DevOPS organization for which the pipeline will be created.

```infrastructure.pipeline.repository``` The name of the repository inside the Azure DevOPS project for which the pipeline will be created.

```infrastructure.pipeline.branch``` The name of the branch that the pipeline should be based on. Use trigger: none to avoid triggering the pipeline on push/pr unless you have purchased parallelization, in which case you don't even need migr8.

```infrastructure.settings``` An array of ```name``` - ```value``` objects that represent the different environment variables of each service. Each application type, has a different way of setting the environment variables. Azure Functions use an ```az cli``` command whereas WebApps integrate them in their ```yaml``` pipeline.

<h3 style="text-decoration:underline;">INFRASTRUCTURE INSTRUCTIONS AND REMARKS</h3>

<p>In order create any infrastructure (Function Apps & WebApps for now) you need to have installed:</p>

<ul>
    <li>az cli</li>
    <li>docker</li>
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

<h2 style="text-decoration:underline;">NODEJS AZURE FUNCTIONS YAML TEMPLATE</h2>

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
    default: '' 
  - name: appName
    type: string
    default: 'default'
  - name: resourceGroup
    type: string
    default: 'default'

variables:
  _agentPool: ${{ parameters.agentPool }}
  _agent: ${{ parameters.agent }}
  _azSub: ${{ parameters.azureSubscription }}

stages:
- stage: Build
  displayName: Build stage
  jobs:
  - job: Build
    displayName: Build
    pool:
      name: $(_agentPool)
      demands:
      - agent.name -equals $(_agent)

    steps:
    - task: NodeTool@0
      inputs:
        versionSpec: '10.x'
      displayName: 'Install Node.js'

    - script: |
        if [ -f extensions.csproj ]
        then
            dotnet build extensions.csproj --runtime ubuntu.16.04-x64 --output ./bin
        fi
      displayName: 'Build extensions'

    - script: |
        npm install
        npm run build --if-present
        npm run test --if-present
      displayName: 'Prepare binaries'

    - task: ArchiveFiles@2
      displayName: 'Archive files'
      inputs:
        rootFolderOrFile: '$(System.DefaultWorkingDirectory)'
        includeRootFolder: false
        archiveType: zip
        archiveFile: $(Build.ArtifactStagingDirectory)/$(Build.BuildId).zip
        replaceExistingArchive: true

    - upload: $(Build.ArtifactStagingDirectory)/$(Build.BuildId).zip
      artifact: drop

- stage: Deploy
  displayName: Deploy stage
  dependsOn: Build
  condition: succeeded()
  jobs:
  - deployment: Deploy
    displayName: Deploy
    environment: ${{ parameters.appName }}
    pool:
      name: $(_agentPool)
      demands:
      - agent.name -equals $(_agent)
    strategy:
      runOnce:
        deploy:
          steps:
          - task: AzureFunctionApp@1
            displayName: 'Azure Functions NodeJS deploy'
            inputs:
              azureSubscription: '$(_azSub)'
              appType: functionAppLinux
              appName: ${{ parameters.appName }}
              package: '$(Pipeline.Workspace)/drop/$(Build.BuildId).zip'
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
        AppSettings: '-NODE_ENV ${{ parameters.NODE_ENV }}'
        # remove the lines below if you dont have post install needs
        ScriptType: 'File Path'
        ScriptPath: '$(System.DefaultWorkingDirectory)/post_install_script.bat' # post_install_script.bat should be part of your project in Azure Git

```

<hr/>

<h1 style="text-decoration:underline;">CLI MODES</h1>

<h2 style="text-decoration:underline;">Help</h2>

``migr8 -h`` - [Shows help information]


<hr/>

``migr8 infra`` with three available modes:

```complete``` Creates infrastructure and deploys based on the ```pipeline``` object

```create``` Only creates the infrastructure. The ```pipeline``` object can be omitted

```deploy``` Only deploys based on the ```pipeline``` object

### Flags

``-i`` The absolute path to an infrastructure configuration file. See example below


### Examples


#### Create and deploy infrastructure

```migr8 infra complete -i C:\Users\test-stack.json```


#### Create infrastructure

```migr8 infra create -i C:\Users\test-stack.json```


#### Deploy source code to infrastructure

```migr8 infra deploy -i C:\Users\test-stack.json```


<hr>

## Service Connections

<p>
    In order to grant access to Azure DevOPS to handle deployments on different Azure resources, you need to create a service connection. Service connections are created per project.
    Follow the below steps to create one:
</p>

devops org -> project -> project settings -> service connections -> new service connection with details:
- azure resource manager (next)
- service principal (automatic) (next)
- Subscription
- Select Subscription from dropdown
- Leave Resource Group Empty
- Service Connection Name = Subscription Id (this convention is important for now)
- Grant access permission to all pipelines

<hr>

## Self Hosted Agent Pools

<p>The ability to have Self Hosted Agent Pools for pipelines is what makes possible the free parallelization of deployment jobs without having to maintain the infrastructure hosting the agent. Follow the below steps to create one:</p>

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
