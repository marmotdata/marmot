# marmot.generated.SsoApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**delete_sso_team_mappings_id**](SsoApi.md#delete_sso_team_mappings_id) | **DELETE** /api/v1/sso/team-mappings/{id} | Delete an SSO team mapping
[**get_sso_team_mappings**](SsoApi.md#get_sso_team_mappings) | **GET** /api/v1/sso/team-mappings | List SSO team mappings
[**get_sso_team_mappings_id**](SsoApi.md#get_sso_team_mappings_id) | **GET** /api/v1/sso/team-mappings/{id} | Get an SSO team mapping
[**post_sso_team_mappings**](SsoApi.md#post_sso_team_mappings) | **POST** /api/v1/sso/team-mappings | Create an SSO team mapping
[**put_sso_team_mappings_id**](SsoApi.md#put_sso_team_mappings_id) | **PUT** /api/v1/sso/team-mappings/{id} | Update an SSO team mapping


# **delete_sso_team_mappings_id**
> MessageResponse delete_sso_team_mappings_id(id)

**Synchronous variant:** `delete_sso_team_mappings_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete an SSO team mapping

Delete an SSO team mapping by its ID

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.message_response import MessageResponse
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
    api_instance = marmot.generated.SsoApi(api_client)
    id = 'id_example' # str | SSO mapping ID

    try:
        # Delete an SSO team mapping
        api_response = await api_instance.delete_sso_team_mappings_id(id)
        print("The response of SsoApi->delete_sso_team_mappings_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling SsoApi->delete_sso_team_mappings_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| SSO mapping ID | 

### Return type

[**MessageResponse**](MessageResponse.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **get_sso_team_mappings**
> ListSSOMappingsResponse get_sso_team_mappings(provider=provider)

**Synchronous variant:** `get_sso_team_mappings_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List SSO team mappings

Get a list of SSO group to team mappings

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.list_sso_mappings_response import ListSSOMappingsResponse
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
    api_instance = marmot.generated.SsoApi(api_client)
    provider = 'provider_example' # str | Filter by SSO provider (optional)

    try:
        # List SSO team mappings
        api_response = await api_instance.get_sso_team_mappings(provider=provider)
        print("The response of SsoApi->get_sso_team_mappings:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling SsoApi->get_sso_team_mappings: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **provider** | **str**| Filter by SSO provider | [optional] 

### Return type

[**ListSSOMappingsResponse**](ListSSOMappingsResponse.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **get_sso_team_mappings_id**
> SSOTeamMapping get_sso_team_mappings_id(id)

**Synchronous variant:** `get_sso_team_mappings_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get an SSO team mapping

Get an SSO team mapping by its ID

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.sso_team_mapping import SSOTeamMapping
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
    api_instance = marmot.generated.SsoApi(api_client)
    id = 'id_example' # str | SSO mapping ID

    try:
        # Get an SSO team mapping
        api_response = await api_instance.get_sso_team_mappings_id(id)
        print("The response of SsoApi->get_sso_team_mappings_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling SsoApi->get_sso_team_mappings_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| SSO mapping ID | 

### Return type

[**SSOTeamMapping**](SSOTeamMapping.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **post_sso_team_mappings**
> SSOTeamMapping post_sso_team_mappings(create_sso_mapping_request)

**Synchronous variant:** `post_sso_team_mappings_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Create an SSO team mapping

Create a new SSO group to team mapping

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.create_sso_mapping_request import CreateSSOMappingRequest
from marmot.generated.models.sso_team_mapping import SSOTeamMapping
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
    api_instance = marmot.generated.SsoApi(api_client)
    create_sso_mapping_request = marmot.generated.CreateSSOMappingRequest() # CreateSSOMappingRequest | SSO mapping creation request

    try:
        # Create an SSO team mapping
        api_response = await api_instance.post_sso_team_mappings(create_sso_mapping_request)
        print("The response of SsoApi->post_sso_team_mappings:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling SsoApi->post_sso_team_mappings: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **create_sso_mapping_request** | [**CreateSSOMappingRequest**](CreateSSOMappingRequest.md)| SSO mapping creation request | 

### Return type

[**SSOTeamMapping**](SSOTeamMapping.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**201** | Created |  -  |
**400** | Bad Request |  -  |
**409** | Conflict |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **put_sso_team_mappings_id**
> MessageResponse put_sso_team_mappings_id(id, update_sso_mapping_request)

**Synchronous variant:** `put_sso_team_mappings_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Update an SSO team mapping

Update an SSO team mapping by its ID

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.message_response import MessageResponse
from marmot.generated.models.update_sso_mapping_request import UpdateSSOMappingRequest
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
    api_instance = marmot.generated.SsoApi(api_client)
    id = 'id_example' # str | SSO mapping ID
    update_sso_mapping_request = marmot.generated.UpdateSSOMappingRequest() # UpdateSSOMappingRequest | SSO mapping update request

    try:
        # Update an SSO team mapping
        api_response = await api_instance.put_sso_team_mappings_id(id, update_sso_mapping_request)
        print("The response of SsoApi->put_sso_team_mappings_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling SsoApi->put_sso_team_mappings_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| SSO mapping ID | 
 **update_sso_mapping_request** | [**UpdateSSOMappingRequest**](UpdateSSOMappingRequest.md)| SSO mapping update request | 

### Return type

[**MessageResponse**](MessageResponse.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**400** | Bad Request |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

