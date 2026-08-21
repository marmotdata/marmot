# marmot.generated.AuthApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**get_auth_provider_callback**](AuthApi.md#get_auth_provider_callback) | **GET** /auth/{provider}/callback | Handle OAuth callback
[**get_auth_provider_login**](AuthApi.md#get_auth_provider_login) | **GET** /auth/{provider}/login | Initiate OAuth login
[**get_auth_providers**](AuthApi.md#get_auth_providers) | **GET** /auth-providers | Get auth configuration
[**get_sso_providers**](AuthApi.md#get_sso_providers) | **GET** /api/v1/sso-providers | List configured SSO providers (admin)
[**post_oauth_token**](AuthApi.md#post_oauth_token) | **POST** /oauth/token | OAuth token endpoint


# **get_auth_provider_callback**
> get_auth_provider_callback(provider, code, state)

**Synchronous variant:** `get_auth_provider_callback_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Handle OAuth callback

Processes the OAuth callback from any provider

### Example


```python
import marmot.generated
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to http://localhost
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "http://localhost"
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
        await api_instance.get_auth_provider_callback(provider, code, state)
    except Exception as e:
        print("Exception when calling AuthApi->get_auth_provider_callback: %s\n" % e)
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

# **get_auth_provider_login**
> get_auth_provider_login(provider)

**Synchronous variant:** `get_auth_provider_login_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Initiate OAuth login

Redirects the user to the OAuth provider for authentication

### Example


```python
import marmot.generated
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to http://localhost
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "http://localhost"
)


# Enter a context with an instance of the API client
async with marmot.generated.ApiClient(configuration) as api_client:
    # Create an instance of the API class
    api_instance = marmot.generated.AuthApi(api_client)
    provider = 'provider_example' # str | OAuth provider (okta, google, github, etc.)

    try:
        # Initiate OAuth login
        await api_instance.get_auth_provider_login(provider)
    except Exception as e:
        print("Exception when calling AuthApi->get_auth_provider_login: %s\n" % e)
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

# **get_auth_providers**
> AuthConfig get_auth_providers()

**Synchronous variant:** `get_auth_providers_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get auth configuration

Returns the enabled auth providers without sensitive data

### Example


```python
import marmot.generated
from marmot.generated.models.auth_config import AuthConfig
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to http://localhost
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "http://localhost"
)


# Enter a context with an instance of the API client
async with marmot.generated.ApiClient(configuration) as api_client:
    # Create an instance of the API class
    api_instance = marmot.generated.AuthApi(api_client)

    try:
        # Get auth configuration
        api_response = await api_instance.get_auth_providers()
        print("The response of AuthApi->get_auth_providers:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AuthApi->get_auth_providers: %s\n" % e)
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

# **get_sso_providers**
> SSOProvidersResponse get_sso_providers()

**Synchronous variant:** `get_sso_providers_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List configured SSO providers (admin)

Read-only view of SSO providers wired via server config. Editing is done in config.yaml.

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.sso_providers_response import SSOProvidersResponse
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to http://localhost
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "http://localhost"
)

# The client must configure the authentication and authorization parameters
# in accordance with the API server security policy.
# Examples for each auth method are provided below, use the example that
# satisfies your auth use case.

# Configure API key authorization: ApiKeyAuth
configuration.api_key['ApiKeyAuth'] = os.environ["API_KEY"]

# Uncomment below to setup prefix (e.g. Bearer) for API key, if needed
# configuration.api_key_prefix['ApiKeyAuth'] = 'Bearer'

# Configure API key authorization: BearerAuth
configuration.api_key['BearerAuth'] = os.environ["API_KEY"]

# Uncomment below to setup prefix (e.g. Bearer) for API key, if needed
# configuration.api_key_prefix['BearerAuth'] = 'Bearer'

# Enter a context with an instance of the API client
async with marmot.generated.ApiClient(configuration) as api_client:
    # Create an instance of the API class
    api_instance = marmot.generated.AuthApi(api_client)

    try:
        # List configured SSO providers (admin)
        api_response = await api_instance.get_sso_providers()
        print("The response of AuthApi->get_sso_providers:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AuthApi->get_sso_providers: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**SSOProvidersResponse**](SSOProvidersResponse.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **post_oauth_token**
> TokenExchangeResponse post_oauth_token(grant_type, subject_token=subject_token, subject_token_type=subject_token_type)

**Synchronous variant:** `post_oauth_token_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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

# Defining the host is optional and defaults to http://localhost
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "http://localhost"
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
        api_response = await api_instance.post_oauth_token(grant_type, subject_token=subject_token, subject_token_type=subject_token_type)
        print("The response of AuthApi->post_oauth_token:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AuthApi->post_oauth_token: %s\n" % e)
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

