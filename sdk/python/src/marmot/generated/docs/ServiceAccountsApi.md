# marmot.generated.ServiceAccountsApi

All URIs are relative to */api/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**service_accounts_get**](ServiceAccountsApi.md#service_accounts_get) | **GET** /service-accounts | List service accounts
[**service_accounts_id_api_keys_get**](ServiceAccountsApi.md#service_accounts_id_api_keys_get) | **GET** /service-accounts/{id}/api-keys | List API keys for a service account
[**service_accounts_id_api_keys_key_id_delete**](ServiceAccountsApi.md#service_accounts_id_api_keys_key_id_delete) | **DELETE** /service-accounts/{id}/api-keys/{keyId} | Delete an API key
[**service_accounts_id_api_keys_post**](ServiceAccountsApi.md#service_accounts_id_api_keys_post) | **POST** /service-accounts/{id}/api-keys | Create API key for a service account
[**service_accounts_id_delete**](ServiceAccountsApi.md#service_accounts_id_delete) | **DELETE** /service-accounts/{id} | Delete service account
[**service_accounts_id_get**](ServiceAccountsApi.md#service_accounts_id_get) | **GET** /service-accounts/{id} | Get service account
[**service_accounts_id_patch**](ServiceAccountsApi.md#service_accounts_id_patch) | **PATCH** /service-accounts/{id} | Update service account
[**service_accounts_post**](ServiceAccountsApi.md#service_accounts_post) | **POST** /service-accounts | Create service account


# **service_accounts_get**
> List[ServiceAccount] service_accounts_get()

**Synchronous variant:** `service_accounts_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List service accounts

Get all service accounts

### Example


```python
import marmot.generated
from marmot.generated.models.service_account import ServiceAccount
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
    api_instance = marmot.generated.ServiceAccountsApi(api_client)

    try:
        # List service accounts
        api_response = await api_instance.service_accounts_get()
        print("The response of ServiceAccountsApi->service_accounts_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ServiceAccountsApi->service_accounts_get: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**List[ServiceAccount]**](ServiceAccount.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **service_accounts_id_api_keys_get**
> List[ServiceAccountAPIKey] service_accounts_id_api_keys_get(id)

**Synchronous variant:** `service_accounts_id_api_keys_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List API keys for a service account

Get all API keys for a service account

### Example


```python
import marmot.generated
from marmot.generated.models.service_account_api_key import ServiceAccountAPIKey
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
    api_instance = marmot.generated.ServiceAccountsApi(api_client)
    id = 'id_example' # str | Service account ID

    try:
        # List API keys for a service account
        api_response = await api_instance.service_accounts_id_api_keys_get(id)
        print("The response of ServiceAccountsApi->service_accounts_id_api_keys_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ServiceAccountsApi->service_accounts_id_api_keys_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Service account ID | 

### Return type

[**List[ServiceAccountAPIKey]**](ServiceAccountAPIKey.md)

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

# **service_accounts_id_api_keys_key_id_delete**
> service_accounts_id_api_keys_key_id_delete(id, key_id)

**Synchronous variant:** `service_accounts_id_api_keys_key_id_delete_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete an API key

Delete an API key for a service account

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
    api_instance = marmot.generated.ServiceAccountsApi(api_client)
    id = 'id_example' # str | Service account ID
    key_id = 'key_id_example' # str | API key ID

    try:
        # Delete an API key
        await api_instance.service_accounts_id_api_keys_key_id_delete(id, key_id)
    except Exception as e:
        print("Exception when calling ServiceAccountsApi->service_accounts_id_api_keys_key_id_delete: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Service account ID | 
 **key_id** | **str**| API key ID | 

### Return type

void (empty response body)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: */*

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**204** | No Content |  -  |
**404** | Not Found |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **service_accounts_id_api_keys_post**
> ServiceAccountAPIKey service_accounts_id_api_keys_post(id, create_service_account_api_key_request)

**Synchronous variant:** `service_accounts_id_api_keys_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Create API key for a service account

Create a new API key. The plaintext key is only returned once.

### Example


```python
import marmot.generated
from marmot.generated.models.create_service_account_api_key_request import CreateServiceAccountAPIKeyRequest
from marmot.generated.models.service_account_api_key import ServiceAccountAPIKey
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
    api_instance = marmot.generated.ServiceAccountsApi(api_client)
    id = 'id_example' # str | Service account ID
    create_service_account_api_key_request = marmot.generated.CreateServiceAccountAPIKeyRequest() # CreateServiceAccountAPIKeyRequest | API key details

    try:
        # Create API key for a service account
        api_response = await api_instance.service_accounts_id_api_keys_post(id, create_service_account_api_key_request)
        print("The response of ServiceAccountsApi->service_accounts_id_api_keys_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ServiceAccountsApi->service_accounts_id_api_keys_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Service account ID | 
 **create_service_account_api_key_request** | [**CreateServiceAccountAPIKeyRequest**](CreateServiceAccountAPIKeyRequest.md)| API key details | 

### Return type

[**ServiceAccountAPIKey**](ServiceAccountAPIKey.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**201** | Created |  -  |
**400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **service_accounts_id_delete**
> service_accounts_id_delete(id)

**Synchronous variant:** `service_accounts_id_delete_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete service account

Soft-delete a service account

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
    api_instance = marmot.generated.ServiceAccountsApi(api_client)
    id = 'id_example' # str | Service account ID

    try:
        # Delete service account
        await api_instance.service_accounts_id_delete(id)
    except Exception as e:
        print("Exception when calling ServiceAccountsApi->service_accounts_id_delete: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Service account ID | 

### Return type

void (empty response body)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: */*

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**204** | No Content |  -  |
**404** | Not Found |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **service_accounts_id_get**
> ServiceAccount service_accounts_id_get(id)

**Synchronous variant:** `service_accounts_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get service account

Get a service account by ID

### Example


```python
import marmot.generated
from marmot.generated.models.service_account import ServiceAccount
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
    api_instance = marmot.generated.ServiceAccountsApi(api_client)
    id = 'id_example' # str | Service account ID

    try:
        # Get service account
        api_response = await api_instance.service_accounts_id_get(id)
        print("The response of ServiceAccountsApi->service_accounts_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ServiceAccountsApi->service_accounts_id_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Service account ID | 

### Return type

[**ServiceAccount**](ServiceAccount.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**404** | Not Found |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **service_accounts_id_patch**
> ServiceAccount service_accounts_id_patch(id, update_service_account_request)

**Synchronous variant:** `service_accounts_id_patch_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Update service account

Update a service account

### Example


```python
import marmot.generated
from marmot.generated.models.service_account import ServiceAccount
from marmot.generated.models.update_service_account_request import UpdateServiceAccountRequest
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
    api_instance = marmot.generated.ServiceAccountsApi(api_client)
    id = 'id_example' # str | Service account ID
    update_service_account_request = marmot.generated.UpdateServiceAccountRequest() # UpdateServiceAccountRequest | Update fields

    try:
        # Update service account
        api_response = await api_instance.service_accounts_id_patch(id, update_service_account_request)
        print("The response of ServiceAccountsApi->service_accounts_id_patch:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ServiceAccountsApi->service_accounts_id_patch: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Service account ID | 
 **update_service_account_request** | [**UpdateServiceAccountRequest**](UpdateServiceAccountRequest.md)| Update fields | 

### Return type

[**ServiceAccount**](ServiceAccount.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**404** | Not Found |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **service_accounts_post**
> ServiceAccount service_accounts_post(create_service_account_request)

**Synchronous variant:** `service_accounts_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Create service account

Create a new service account

### Example


```python
import marmot.generated
from marmot.generated.models.create_service_account_request import CreateServiceAccountRequest
from marmot.generated.models.service_account import ServiceAccount
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
    api_instance = marmot.generated.ServiceAccountsApi(api_client)
    create_service_account_request = marmot.generated.CreateServiceAccountRequest() # CreateServiceAccountRequest | Service account

    try:
        # Create service account
        api_response = await api_instance.service_accounts_post(create_service_account_request)
        print("The response of ServiceAccountsApi->service_accounts_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ServiceAccountsApi->service_accounts_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **create_service_account_request** | [**CreateServiceAccountRequest**](CreateServiceAccountRequest.md)| Service account | 

### Return type

[**ServiceAccount**](ServiceAccount.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**201** | Created |  -  |
**400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

