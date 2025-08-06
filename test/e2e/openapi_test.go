package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/marmotdata/marmot/tests/e2e/internal/client/client/assets"
	"github.com/marmotdata/marmot/tests/e2e/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAPIIngestion(t *testing.T) {
	specsDir, err := filepath.Abs("/tmp/openapi-specs")
	require.NoError(t, err, "Failed to get absolute path")

	if err := os.RemoveAll(specsDir); err != nil && !os.IsNotExist(err) {
		require.NoError(t, err, "Failed to clean up existing specs directory")
	}
	require.NoError(t, os.MkdirAll(specsDir, 0777), "Failed to create specs directory")

	openapiSpec := `
openapi: 3.0.2
info:
  version: '1.0.0' # Your API version
  # It can be any string but it is better to use semantic versioning: http://semver.org/
  # Warning: OpenAPI requires the version to be a string, but without quotation marks YAML can recognize it as a number.
  
  title: Example.com # Replace with your API title
  # Keep it simple. Don't add "API" or version at the end of the string.

  termsOfService: 'https://example.com/terms/' # [Optional] Replace with an URL to your ToS
  contact:
    email: contact@example.com # [Optional] Replace with your contact email
    url: 'http://example.com/contact' # [Optional] Replace with link to your contact form
  license:
    name: Apache 2.0
    url: 'http://www.apache.org/licenses/LICENSE-2.0.html'
  x-logo:
    url: 'https://redocly.github.io/openapi-template/logo.png'
  
  # Describe your API here, you can use GFM (https://guides.github.com/features/mastering-markdown) here
  description: |
    This is an **example** API to demonstrate features of OpenAPI specification
    # Introduction
    This API definition is intended to to be a good starting point for describing your API in 
    [OpenAPI/Swagger format](https://github.com/OAI/OpenAPI-Specification/blob/master/versions/3.0.2.md).
    It also demonstrates features of [create-openapi-repo](https://github.com/Redocly/create-openapi-repo) tool and 
    [Redoc](https://github.com/Redocly/Redoc) documentation engine. So beyond the standard OpenAPI syntax we use a few 
    [vendor extensions](https://github.com/Redocly/Redoc/blob/master/docs/redoc-vendor-extensions.md).

    # OpenAPI Specification
    The goal of The OpenAPI Specification is to define a standard, language-agnostic interface to REST APIs which
    allows both humans and computers to discover and understand the capabilities of the service without access to source
    code, documentation, or through network traffic inspection. When properly defined via OpenAPI, a consumer can 
    understand and interact with the remote service with a minimal amount of implementation logic. Similar to what
    interfaces have done for lower-level programming, OpenAPI removes the guesswork in calling the service.
externalDocs:
  description: Find out how to create a GitHub repo for your OpenAPI definition.
  url: 'https://github.com/Rebilly/generator-openapi-repo'

# A list of tags used by the definition with additional metadata.
# The order of the tags can be used to reflect on their order by the parsing tools.
tags:
  - name: Echo
    description: Example echo operations
  - name: User
    description: Operations about user
servers:
  - url: 'http://example.com/api/v1'
  - url: 'https://example.com/api/v1'

# Holds the relative paths to the individual endpoints. The path is appended to the
# basePath in order to construct the full URL. 
paths:
  '/users/{username}': # path parameter in curly braces

    # parameters list that are used with each operation for this path
    parameters:
      - name: pretty_print
        in: query
        description: Pretty print response
        schema:
          type: boolean
    get: # documentation for GET operation for this path
      tags:
        - User
      
      # summary is up to 120 symbold but we recommend to be shortest as possible
      summary: Get user by user name
      
      # you can use GFM in operation description too: https://guides.github.com/features/mastering-markdown
      description: |
        Some description of the operation. 
      
      # operationId should be unique across the whole specification
      operationId: getUserByName
      
      # list of parameters for the operation
      parameters:
        - name: username
          in: path
          description: The name that needs to be fetched
          required: true
          schema:
            type: string
        - name: with_email
          in: query
          description: Filter users without email
          schema:
            type: boolean
      
      # security schemas applied to this operation
      security:
        - main_auth:
            - 'read:users' # for oauth2 provide list of scopes here
        - api_key: []
      responses: # list of responses
        '200':
          description: Success
          content:
            application/json: # operation response mime type
              schema: # response schema can be specified for each response
                $ref: '#/components/schemas/User'
              example: # response example
                username: user1
                email: user@example.com
        '403':
          description: Forbidden
        '404':
          description: User not found
    # documentation for PUT operation for this path
    put:
      tags:
        - User
      summary: Updated user
      description: This can only be done by the logged in user.
      operationId: updateUser
      parameters:
        - name: username
          in: path
          description: The name that needs to be updated
          required: true
          schema:
            type: string
      security:
        - main_auth:
            - 'write:users'
      responses:
        '200':
          description: OK
        '400':
          description: Invalid user supplied
        '404':
          description: User not found
      # request body documentation
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/User'
          application/xml:
            schema:
              $ref: '#/components/schemas/User'
        description: Updated user object
        required: true
  /echo: # path parameter in curly braces
    post: # documentation for POST operation for this path
      tags:
        - Echo
      summary: Echo test
      description: Receive the exact message you've sent
      operationId: echo
      security:
        - api_key: []
        - basic_auth: []
      responses:
        '200':
          description: OK
          # document headers for this response
          headers:
            X-Rate-Limit: # Header name
              description: calls per hour allowed by the user
              schema: # Header schema
                type: integer
                format: int32
            X-Expires-After:
              $ref: '#/components/headers/ExpiresAfter'
          content:
            application/json:
              schema:
                type: string
              examples:
                response:
                  value: Hello world!
            application/xml:
              schema:
                type: string
            text/csv:
              schema:
                type: string
      requestBody:
        content:
          application/json:
            schema:
              type: string
              example: Hello world!
          application/xml:
            schema:
              type: string
              example: Hello world!
        description: Echo payload
        required: true
        
# An object to hold reusable parts that can be used across the definition
components:
  schemas:
    Email:
      description: User email address
      type: string
      format: test
      example: john.smith@example.com
    User:
      type: object
      properties:
        username:
          description: User supplied username
          type: string
          minLength: 4
          example: John78
        firstName:
          description: User first name
          type: string
          minLength: 1
          example: John
        lastName:
          description: User last name
          type: string
          minLength: 1
          example: Smith
        email:
          $ref: '#/components/schemas/Email'
  headers:
    ExpiresAfter:
      description: date in UTC when token expires
      schema:
        type: string
        format: date-time
  # Security scheme definitions that can be used across the definition.
  securitySchemes:
    main_auth: # security definition name (you can name it as you want)
      # the following options are specific to oauth2 type
      type: oauth2 # authorization type, one of: oauth2, apiKey, http
      flows:
        implicit:
          authorizationUrl: 'http://example.com/api/oauth/dialog'
          scopes:
            'read:users': read users info
            'write:users': modify or remove users
    api_key:  # security definition name (you can name it as you want)
      type: apiKey 
      # The following options are specific to apiKey type
      in: header # Where API key will be passed: header or query
      name: api_key # API key parameter name
    basic_auth: # security definition name (you can name it as you want)
      type: http
      scheme: basic
`

	openapiSpecPath := filepath.Join(specsDir, "openapi.yaml")
	require.NoError(t, os.WriteFile(openapiSpecPath, []byte(openapiSpec), 0666))

	files, err := os.ReadDir(specsDir)
	require.NoError(t, err, "Error reading spec directory")
	for _, file := range files {
		fileInfo, err := os.Stat(filepath.Join(specsDir, file.Name()))
		require.NoError(t, err)
		t.Logf("Spec file found: %s (size: %d, mode: %s)",
			file.Name(), fileInfo.Size(), fileInfo.Mode().String())
	}

	t.Logf("Contents of %s:", specsDir)
	for _, file := range files {
		t.Logf("  - %s", file.Name())
	}

	testFile := filepath.Join(specsDir, "test-file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0666))

	configContent := fmt.Sprintf(`
runs:
  - openapi:
      spec_path: "/tmp/openapi-specs"
      tags:
        - "openapi"
        - "api"
`)

	err = env.ContainerManager.RunMarmotCommandWithConfig(env.Config,
		[]string{"ingest", "-c", "/tmp/config.yaml", "-k", env.APIKey,
			"-H", "http://marmot-test:8080"},
		configContent,
		nil,
		fmt.Sprintf("%s:/tmp/opencapi-specs", specsDir),
	)
	require.NoError(t, err)

	debugCmd := []string{"ls", "-la", "/tmp/openapi-specs"}
	containerConfig := &container.Config{
		Image: "marmot:test",
		Cmd:   debugCmd,
	}
	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(env.Config.NetworkName),
		Binds:       []string{fmt.Sprintf("%s:/tmp/openapi-specs", specsDir)},
	}
	debugContainerID, err := env.ContainerManager.StartContainer(containerConfig, hostConfig, "")
	require.NoError(t, err)
	defer env.ContainerManager.CleanupContainer(debugContainerID)

	debugOutput, err := env.ContainerManager.ExecCommand(debugContainerID, []string{"cat", "/tmp/openapi-specs/openapi.yaml"})
	t.Logf("Debug container output: %s", debugOutput)

	t.Log("Ingest command executed, waiting for assets...")

	var resp *assets.GetAssetsListOK
	found := false

	for i := 0; i < 10; i++ {
		time.Sleep(3 * time.Second)

		params := assets.NewGetAssetsListParams()
		resp, err = env.APIClient.Assets.GetAssetsList(params)
		require.NoError(t, err)

		if len(resp.Payload.Assets) > 0 {
			found = true
			break
		}

		t.Logf("No assets found yet (attempt %d/10)", i+1)
	}

	require.True(t, found, "No assets found after multiple attempts")

	openapiService := utils.FindAssetByName(resp.Payload.Assets, "Example.com")
	require.NotNil(t, openapiService, "OpenAPI service not found")
	assert.Equal(t, "API", openapiService.Type)
	assert.Contains(t, openapiService.Tags, "openapi")
	assert.Contains(t, openapiService.Tags, "api")
	assert.Contains(t, openapiService.Tags, "Echo")
	assert.Contains(t, openapiService.Tags, "User")
	assert.Equal(t, len(openapiService.Tags), 4)

	t.Log("Cleaning up created assets...")
	assetIDs := []string{
		openapiService.ID,
	}

	for _, id := range assetIDs {
		_, err := env.APIClient.Assets.DeleteAssetsID(assets.NewDeleteAssetsIDParams().WithID(id))
		assert.NoError(t, err, "Failed to delete asset %s", id)
	}
}
