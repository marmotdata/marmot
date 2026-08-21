# marmot.generated.ServiceAccountsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**delete_service_accounts_id**](ServiceAccountsApi.md#delete_service_accounts_id) | **DELETE** /api/v1/service-accounts/{id} | Delete service account
[**delete_service_accounts_idapi_keys_key_id**](ServiceAccountsApi.md#delete_service_accounts_idapi_keys_key_id) | **DELETE** /api/v1/service-accounts/{id}/api-keys/{keyId} | Delete an API key
[**get_service_accounts**](ServiceAccountsApi.md#get_service_accounts) | **GET** /api/v1/service-accounts | List service accounts
[**get_service_accounts_id**](ServiceAccountsApi.md#get_service_accounts_id) | **GET** /api/v1/service-accounts/{id} | Get service account
[**get_service_accounts_idapi_keys**](ServiceAccountsApi.md#get_service_accounts_idapi_keys) | **GET** /api/v1/service-accounts/{id}/api-keys | List API keys for a service account
[**patch_service_accounts_id**](ServiceAccountsApi.md#patch_service_accounts_id) | **PATCH** /api/v1/service-accounts/{id} | Update service account
[**post_service_accounts**](ServiceAccountsApi.md#post_service_accounts) | **POST** /api/v1/service-accounts | Create service account
[**post_service_accounts_idapi_keys**](ServiceAccountsApi.md#post_service_accounts_idapi_keys) | **POST** /api/v1/service-accounts/{id}/api-keys | Create API key for a service account


# **delete_service_accounts_id**
> delete_service_accounts_id(id)

**Synchronous variant:** `delete_service_accounts_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete service account

Soft-delete a service account

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
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
    api_instance = marmot.generated.ServiceAccountsApi(api_client)
    id = 'id_example' # str | Service account ID

    try:
        # Delete service account
        await api_instance.delete_service_accounts_id(id)
    except Exception as e:
        print("Exception when calling ServiceAccountsApi->delete_service_accounts_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Service account ID | 

### Return type

void (empty response body)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: */*

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**204** | No Content |  -  |
**404** | Not Found |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **delete_service_accounts_idapi_keys_key_id**
> delete_service_accounts_idapi_keys_key_id(id, key_id)

**Synchronous variant:** `delete_service_accounts_idapi_keys_key_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete an API key

Delete an API key for a service account

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
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
    api_instance = marmot.generated.ServiceAccountsApi(api_client)
    id = 'id_example' # str | Service account ID
    key_id = 'key_id_example' # str | API key ID

    try:
        # Delete an API key
        await api_instance.delete_service_accounts_idapi_keys_key_id(id, key_id)
    except Exception as e:
        print("Exception when calling ServiceAccountsApi->delete_service_accounts_idapi_keys_key_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Service account ID | 
 **key_id** | **str**| API key ID | 

### Return type

void (empty response body)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: */*

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**204** | No Content |  -  |
**404** | Not Found |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **get_service_accounts**
> List[ServiceAccount] get_service_accounts()

**Synchronous variant:** `get_service_accounts_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List service accounts

Get all service accounts

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.service_account import ServiceAccount
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
    api_instance = marmot.generated.ServiceAccountsApi(api_client)

    try:
        # List service accounts
        api_response = await api_instance.get_service_accounts()
        print("The response of ServiceAccountsApi->get_service_accounts:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ServiceAccountsApi->get_service_accounts: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**List[ServiceAccount]**](ServiceAccount.md)

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

# **get_service_accounts_id**
> ServiceAccount get_service_accounts_id(id)

**Synchronous variant:** `get_service_accounts_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get service account

Get a service account by ID

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.service_account import ServiceAccount
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
    api_instance = marmot.generated.ServiceAccountsApi(api_client)
    id = 'id_example' # str | Service account ID

    try:
        # Get service account
        api_response = await api_instance.get_service_accounts_id(id)
        print("The response of ServiceAccountsApi->get_service_accounts_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ServiceAccountsApi->get_service_accounts_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Service account ID | 

### Return type

[**ServiceAccount**](ServiceAccount.md)

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

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **get_service_accounts_idapi_keys**
> List[ServiceAccountAPIKey] get_service_accounts_idapi_keys(id)

**Synchronous variant:** `get_service_accounts_idapi_keys_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List API keys for a service account

Get all API keys for a service account

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.service_account_api_key import ServiceAccountAPIKey
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
    api_instance = marmot.generated.ServiceAccountsApi(api_client)
    id = 'id_example' # str | Service account ID

    try:
        # List API keys for a service account
        api_response = await api_instance.get_service_accounts_idapi_keys(id)
        print("The response of ServiceAccountsApi->get_service_accounts_idapi_keys:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ServiceAccountsApi->get_service_accounts_idapi_keys: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Service account ID | 

### Return type

[**List[ServiceAccountAPIKey]**](ServiceAccountAPIKey.md)

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

# **patch_service_accounts_id**
> ServiceAccount patch_service_accounts_id(id, update_service_account_request)

**Synchronous variant:** `patch_service_accounts_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Update service account

Update a service account

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.service_account import ServiceAccount
from marmot.generated.models.update_service_account_request import UpdateServiceAccountRequest
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
    api_instance = marmot.generated.ServiceAccountsApi(api_client)
    id = 'id_example' # str | Service account ID
    update_service_account_request = marmot.generated.UpdateServiceAccountRequest() # UpdateServiceAccountRequest | Update fields

    try:
        # Update service account
        api_response = await api_instance.patch_service_accounts_id(id, update_service_account_request)
        print("The response of ServiceAccountsApi->patch_service_accounts_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ServiceAccountsApi->patch_service_accounts_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Service account ID | 
 **update_service_account_request** | [**UpdateServiceAccountRequest**](UpdateServiceAccountRequest.md)| Update fields | 

### Return type

[**ServiceAccount**](ServiceAccount.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**404** | Not Found |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **post_service_accounts**
> ServiceAccount post_service_accounts(create_service_account_request)

**Synchronous variant:** `post_service_accounts_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Create service account

Create a new service account

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.create_service_account_request import CreateServiceAccountRequest
from marmot.generated.models.service_account import ServiceAccount
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
    api_instance = marmot.generated.ServiceAccountsApi(api_client)
    create_service_account_request = marmot.generated.CreateServiceAccountRequest() # CreateServiceAccountRequest | Service account

    try:
        # Create service account
        api_response = await api_instance.post_service_accounts(create_service_account_request)
        print("The response of ServiceAccountsApi->post_service_accounts:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ServiceAccountsApi->post_service_accounts: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **create_service_account_request** | [**CreateServiceAccountRequest**](CreateServiceAccountRequest.md)| Service account | 

### Return type

[**ServiceAccount**](ServiceAccount.md)

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

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **post_service_accounts_idapi_keys**
> ServiceAccountAPIKey post_service_accounts_idapi_keys(id, create_service_account_api_key_request)

**Synchronous variant:** `post_service_accounts_idapi_keys_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Create API key for a service account

Create a new API key. The plaintext key is only returned once.

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.create_service_account_api_key_request import CreateServiceAccountAPIKeyRequest
from marmot.generated.models.service_account_api_key import ServiceAccountAPIKey
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
    api_instance = marmot.generated.ServiceAccountsApi(api_client)
    id = 'id_example' # str | Service account ID
    create_service_account_api_key_request = marmot.generated.CreateServiceAccountAPIKeyRequest() # CreateServiceAccountAPIKeyRequest | API key details

    try:
        # Create API key for a service account
        api_response = await api_instance.post_service_accounts_idapi_keys(id, create_service_account_api_key_request)
        print("The response of ServiceAccountsApi->post_service_accounts_idapi_keys:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ServiceAccountsApi->post_service_accounts_idapi_keys: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Service account ID | 
 **create_service_account_api_key_request** | [**CreateServiceAccountAPIKeyRequest**](CreateServiceAccountAPIKeyRequest.md)| API key details | 

### Return type

[**ServiceAccountAPIKey**](ServiceAccountAPIKey.md)

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

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

