# marmot.generated.AuthApi

All URIs are relative to */api/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_provider_callback_get**](AuthApi.md#auth_provider_callback_get) | **GET** /auth/{provider}/callback | Handle OAuth callback
[**auth_provider_login_get**](AuthApi.md#auth_provider_login_get) | **GET** /auth/{provider}/login | Initiate OAuth login
[**auth_providers_get**](AuthApi.md#auth_providers_get) | **GET** /auth-providers | Get auth configuration
[**oauth_token_post**](AuthApi.md#oauth_token_post) | **POST** /oauth/token | OAuth token endpoint
[**sso_providers_get**](AuthApi.md#sso_providers_get) | **GET** /sso-providers | List configured SSO providers (admin)


# **auth_provider_callback_get**
> auth_provider_callback_get(provider, code, state)

**Synchronous variant:** `auth_provider_callback_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Handle OAuth callback

Processes the OAuth callback from any provider

### Example


```python
import marmot.generated
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to /api/v1
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "/api/v1"
)


# Enter a context with an instance of the API client
async with marmot.generated.ApiClient(configuration) as api_client:
    # Create an instance of the API class
    api_instance = marmot.generated.AuthApi(api_client)
    provider = 'provider_example' # str | OAuth provider (okta, google, github, etc.)
    code = 'code_example' # str | Authorization code
    state = 'state_example' # str | State parameter for CSRF protection

    try:
        # Handle OAuth callback
        await api_instance.auth_provider_callback_get(provider, code, state)
    except Exception as e:
        print("Exception when calling AuthApi->auth_provider_callback_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **provider** | **str**| OAuth provider (okta, google, github, etc.) | 
 **code** | **str**| Authorization code | 
 **state** | **str**| State parameter for CSRF protection | 

### Return type

void (empty response body)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**307** | Temporary Redirect |  -  |
**400** | Bad Request |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **auth_provider_login_get**
> auth_provider_login_get(provider)

**Synchronous variant:** `auth_provider_login_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Initiate OAuth login

Redirects the user to the OAuth provider for authentication

### Example


```python
import marmot.generated
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to /api/v1
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "/api/v1"
)


# Enter a context with an instance of the API client
async with marmot.generated.ApiClient(configuration) as api_client:
    # Create an instance of the API class
    api_instance = marmot.generated.AuthApi(api_client)
    provider = 'provider_example' # str | OAuth provider (okta, google, github, etc.)

    try:
        # Initiate OAuth login
        await api_instance.auth_provider_login_get(provider)
    except Exception as e:
        print("Exception when calling AuthApi->auth_provider_login_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **provider** | **str**| OAuth provider (okta, google, github, etc.) | 

### Return type

void (empty response body)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**307** | Temporary Redirect |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **auth_providers_get**
> AuthConfig auth_providers_get()

**Synchronous variant:** `auth_providers_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get auth configuration

Returns the enabled auth providers without sensitive data

### Example


```python
import marmot.generated
from marmot.generated.models.auth_config import AuthConfig
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to /api/v1
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "/api/v1"
)


# Enter a context with an instance of the API client
async with marmot.generated.ApiClient(configuration) as api_client:
    # Create an instance of the API class
    api_instance = marmot.generated.AuthApi(api_client)

    try:
        # Get auth configuration
        api_response = await api_instance.auth_providers_get()
        print("The response of AuthApi->auth_providers_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AuthApi->auth_providers_get: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**AuthConfig**](AuthConfig.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **oauth_token_post**
> TokenExchangeResponse oauth_token_post(grant_type, subject_token=subject_token, subject_token_type=subject_token_type)

**Synchronous variant:** `oauth_token_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

OAuth token endpoint

Handles authorization_code grants (with PKCE) and token exchange (RFC 8693).
For token-exchange, supported subject_token_type values are
urn:ietf:params:oauth:token-type:id_token and urn:ietf:params:oauth:token-type:access_token.

### Example


```python
import marmot.generated
from marmot.generated.models.token_exchange_response import TokenExchangeResponse
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to /api/v1
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "/api/v1"
)


# Enter a context with an instance of the API client
async with marmot.generated.ApiClient(configuration) as api_client:
    # Create an instance of the API class
    api_instance = marmot.generated.AuthApi(api_client)
    grant_type = 'grant_type_example' # str | authorization_code or urn:ietf:params:oauth:grant-type:token-exchange
    subject_token = 'subject_token_example' # str | Token to exchange (token-exchange grant only) (optional)
    subject_token_type = 'subject_token_type_example' # str | id_token or access_token URI (token-exchange grant only) (optional)

    try:
        # OAuth token endpoint
        api_response = await api_instance.oauth_token_post(grant_type, subject_token=subject_token, subject_token_type=subject_token_type)
        print("The response of AuthApi->oauth_token_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AuthApi->oauth_token_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **grant_type** | **str**| authorization_code or urn:ietf:params:oauth:grant-type:token-exchange | 
 **subject_token** | **str**| Token to exchange (token-exchange grant only) | [optional] 
 **subject_token_type** | **str**| id_token or access_token URI (token-exchange grant only) | [optional] 

### Return type

[**TokenExchangeResponse**](TokenExchangeResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/x-www-form-urlencoded
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**400** | Bad Request |  -  |
**401** | Unauthorized |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **sso_providers_get**
> SSOProvidersResponse sso_providers_get()

**Synchronous variant:** `sso_providers_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List configured SSO providers (admin)

Read-only view of SSO providers wired via server config. Editing is done in config.yaml.

### Example


```python
import marmot.generated
from marmot.generated.models.sso_providers_response import SSOProvidersResponse
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to /api/v1
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "/api/v1"
)


# Enter a context with an instance of the API client
async with marmot.generated.ApiClient(configuration) as api_client:
    # Create an instance of the API class
    api_instance = marmot.generated.AuthApi(api_client)

    try:
        # List configured SSO providers (admin)
        api_response = await api_instance.sso_providers_get()
        print("The response of AuthApi->sso_providers_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AuthApi->sso_providers_get: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**SSOProvidersResponse**](SSOProvidersResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

