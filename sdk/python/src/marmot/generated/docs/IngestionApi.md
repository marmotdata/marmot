# marmot.generated.IngestionApi

All URIs are relative to */api/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**ingestion_runs_get**](IngestionApi.md#ingestion_runs_get) | **GET** /ingestion/runs | List ingestion job runs
[**ingestion_runs_id_cancel_post**](IngestionApi.md#ingestion_runs_id_cancel_post) | **POST** /ingestion/runs/{id}/cancel | Cancel a running job
[**ingestion_runs_id_entities_get**](IngestionApi.md#ingestion_runs_id_entities_get) | **GET** /ingestion/runs/{id}/entities | Get entities for a job run
[**ingestion_runs_id_get**](IngestionApi.md#ingestion_runs_id_get) | **GET** /ingestion/runs/{id} | Get a job run by ID
[**ingestion_schedules_get**](IngestionApi.md#ingestion_schedules_get) | **GET** /ingestion/schedules | List ingestion schedules
[**ingestion_schedules_id_delete**](IngestionApi.md#ingestion_schedules_id_delete) | **DELETE** /ingestion/schedules/{id} | Delete an ingestion schedule
[**ingestion_schedules_id_get**](IngestionApi.md#ingestion_schedules_id_get) | **GET** /ingestion/schedules/{id} | Get an ingestion schedule by ID
[**ingestion_schedules_id_put**](IngestionApi.md#ingestion_schedules_id_put) | **PUT** /ingestion/schedules/{id} | Update an ingestion schedule
[**ingestion_schedules_id_trigger_post**](IngestionApi.md#ingestion_schedules_id_trigger_post) | **POST** /ingestion/schedules/{id}/trigger | Manually trigger an ingestion schedule
[**ingestion_schedules_post**](IngestionApi.md#ingestion_schedules_post) | **POST** /ingestion/schedules | Create a new ingestion schedule
[**ingestion_validate_post**](IngestionApi.md#ingestion_validate_post) | **POST** /ingestion/validate | Validate plugin configuration


# **ingestion_runs_get**
> ListJobRunsResponse ingestion_runs_get(schedule_id=schedule_id, status=status, limit=limit, offset=offset)

**Synchronous variant:** `ingestion_runs_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List ingestion job runs

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.list_job_runs_response import ListJobRunsResponse
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to /api/v1
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "/api/v1"
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
    api_instance = marmot.generated.IngestionApi(api_client)
    schedule_id = 'schedule_id_example' # str | Filter by schedule ID (optional)
    status = 'status_example' # str | Filter by status (optional)
    limit = 56 # int | Limit (optional)
    offset = 56 # int | Offset (optional)

    try:
        # List ingestion job runs
        api_response = await api_instance.ingestion_runs_get(schedule_id=schedule_id, status=status, limit=limit, offset=offset)
        print("The response of IngestionApi->ingestion_runs_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling IngestionApi->ingestion_runs_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **schedule_id** | **str**| Filter by schedule ID | [optional] 
 **status** | **str**| Filter by status | [optional] 
 **limit** | **int**| Limit | [optional] 
 **offset** | **int**| Offset | [optional] 

### Return type

[**ListJobRunsResponse**](ListJobRunsResponse.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**401** | Unauthorized |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ingestion_runs_id_cancel_post**
> ingestion_runs_id_cancel_post(id)

**Synchronous variant:** `ingestion_runs_id_cancel_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Cancel a running job

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to /api/v1
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "/api/v1"
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
    api_instance = marmot.generated.IngestionApi(api_client)
    id = 'id_example' # str | Job run ID

    try:
        # Cancel a running job
        await api_instance.ingestion_runs_id_cancel_post(id)
    except Exception as e:
        print("Exception when calling IngestionApi->ingestion_runs_id_cancel_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Job run ID | 

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
**401** | Unauthorized |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ingestion_runs_id_entities_get**
> Dict[str, object] ingestion_runs_id_entities_get(id, limit=limit, offset=offset)

**Synchronous variant:** `ingestion_runs_id_entities_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get entities for a job run

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to /api/v1
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "/api/v1"
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
    api_instance = marmot.generated.IngestionApi(api_client)
    id = 'id_example' # str | Job run ID
    limit = 56 # int | Limit (optional)
    offset = 56 # int | Offset (optional)

    try:
        # Get entities for a job run
        api_response = await api_instance.ingestion_runs_id_entities_get(id, limit=limit, offset=offset)
        print("The response of IngestionApi->ingestion_runs_id_entities_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling IngestionApi->ingestion_runs_id_entities_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Job run ID | 
 **limit** | **int**| Limit | [optional] 
 **offset** | **int**| Offset | [optional] 

### Return type

**Dict[str, object]**

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**401** | Unauthorized |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ingestion_runs_id_get**
> JobRun ingestion_runs_id_get(id)

**Synchronous variant:** `ingestion_runs_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get a job run by ID

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.job_run import JobRun
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to /api/v1
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "/api/v1"
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
    api_instance = marmot.generated.IngestionApi(api_client)
    id = 'id_example' # str | Job run ID

    try:
        # Get a job run by ID
        api_response = await api_instance.ingestion_runs_id_get(id)
        print("The response of IngestionApi->ingestion_runs_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling IngestionApi->ingestion_runs_id_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Job run ID | 

### Return type

[**JobRun**](JobRun.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**401** | Unauthorized |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ingestion_schedules_get**
> ListSchedulesResponse ingestion_schedules_get(enabled=enabled, limit=limit, offset=offset)

**Synchronous variant:** `ingestion_schedules_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List ingestion schedules

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.list_schedules_response import ListSchedulesResponse
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to /api/v1
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "/api/v1"
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
    api_instance = marmot.generated.IngestionApi(api_client)
    enabled = True # bool | Filter by enabled status (optional)
    limit = 56 # int | Limit (optional)
    offset = 56 # int | Offset (optional)

    try:
        # List ingestion schedules
        api_response = await api_instance.ingestion_schedules_get(enabled=enabled, limit=limit, offset=offset)
        print("The response of IngestionApi->ingestion_schedules_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling IngestionApi->ingestion_schedules_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **enabled** | **bool**| Filter by enabled status | [optional] 
 **limit** | **int**| Limit | [optional] 
 **offset** | **int**| Offset | [optional] 

### Return type

[**ListSchedulesResponse**](ListSchedulesResponse.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**401** | Unauthorized |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ingestion_schedules_id_delete**
> ingestion_schedules_id_delete(id)

**Synchronous variant:** `ingestion_schedules_id_delete_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete an ingestion schedule

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to /api/v1
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "/api/v1"
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
    api_instance = marmot.generated.IngestionApi(api_client)
    id = 'id_example' # str | Schedule ID

    try:
        # Delete an ingestion schedule
        await api_instance.ingestion_schedules_id_delete(id)
    except Exception as e:
        print("Exception when calling IngestionApi->ingestion_schedules_id_delete: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Schedule ID | 

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
**401** | Unauthorized |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ingestion_schedules_id_get**
> Schedule ingestion_schedules_id_get(id)

**Synchronous variant:** `ingestion_schedules_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get an ingestion schedule by ID

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.schedule import Schedule
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to /api/v1
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "/api/v1"
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
    api_instance = marmot.generated.IngestionApi(api_client)
    id = 'id_example' # str | Schedule ID

    try:
        # Get an ingestion schedule by ID
        api_response = await api_instance.ingestion_schedules_id_get(id)
        print("The response of IngestionApi->ingestion_schedules_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling IngestionApi->ingestion_schedules_id_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Schedule ID | 

### Return type

[**Schedule**](Schedule.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**401** | Unauthorized |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ingestion_schedules_id_put**
> Schedule ingestion_schedules_id_put(id, update_schedule_request)

**Synchronous variant:** `ingestion_schedules_id_put_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Update an ingestion schedule

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.schedule import Schedule
from marmot.generated.models.update_schedule_request import UpdateScheduleRequest
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to /api/v1
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "/api/v1"
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
    api_instance = marmot.generated.IngestionApi(api_client)
    id = 'id_example' # str | Schedule ID
    update_schedule_request = marmot.generated.UpdateScheduleRequest() # UpdateScheduleRequest | Updated schedule configuration

    try:
        # Update an ingestion schedule
        api_response = await api_instance.ingestion_schedules_id_put(id, update_schedule_request)
        print("The response of IngestionApi->ingestion_schedules_id_put:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling IngestionApi->ingestion_schedules_id_put: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Schedule ID | 
 **update_schedule_request** | [**UpdateScheduleRequest**](UpdateScheduleRequest.md)| Updated schedule configuration | 

### Return type

[**Schedule**](Schedule.md)

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
**401** | Unauthorized |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ingestion_schedules_id_trigger_post**
> JobRun ingestion_schedules_id_trigger_post(id)

**Synchronous variant:** `ingestion_schedules_id_trigger_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Manually trigger an ingestion schedule

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.job_run import JobRun
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to /api/v1
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "/api/v1"
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
    api_instance = marmot.generated.IngestionApi(api_client)
    id = 'id_example' # str | Schedule ID

    try:
        # Manually trigger an ingestion schedule
        api_response = await api_instance.ingestion_schedules_id_trigger_post(id)
        print("The response of IngestionApi->ingestion_schedules_id_trigger_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling IngestionApi->ingestion_schedules_id_trigger_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Schedule ID | 

### Return type

[**JobRun**](JobRun.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: */*

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**201** | Created |  -  |
**401** | Unauthorized |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ingestion_schedules_post**
> Schedule ingestion_schedules_post(create_schedule_request)

**Synchronous variant:** `ingestion_schedules_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Create a new ingestion schedule

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.create_schedule_request import CreateScheduleRequest
from marmot.generated.models.schedule import Schedule
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to /api/v1
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "/api/v1"
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
    api_instance = marmot.generated.IngestionApi(api_client)
    create_schedule_request = marmot.generated.CreateScheduleRequest() # CreateScheduleRequest | Schedule configuration

    try:
        # Create a new ingestion schedule
        api_response = await api_instance.ingestion_schedules_post(create_schedule_request)
        print("The response of IngestionApi->ingestion_schedules_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling IngestionApi->ingestion_schedules_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **create_schedule_request** | [**CreateScheduleRequest**](CreateScheduleRequest.md)| Schedule configuration | 

### Return type

[**Schedule**](Schedule.md)

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
**401** | Unauthorized |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ingestion_validate_post**
> ValidateConfigResponse ingestion_validate_post(validate_config_request)

**Synchronous variant:** `ingestion_validate_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Validate plugin configuration

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.validate_config_request import ValidateConfigRequest
from marmot.generated.models.validate_config_response import ValidateConfigResponse
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to /api/v1
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "/api/v1"
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
    api_instance = marmot.generated.IngestionApi(api_client)
    validate_config_request = marmot.generated.ValidateConfigRequest() # ValidateConfigRequest | Config to validate

    try:
        # Validate plugin configuration
        api_response = await api_instance.ingestion_validate_post(validate_config_request)
        print("The response of IngestionApi->ingestion_validate_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling IngestionApi->ingestion_validate_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **validate_config_request** | [**ValidateConfigRequest**](ValidateConfigRequest.md)| Config to validate | 

### Return type

[**ValidateConfigResponse**](ValidateConfigResponse.md)

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
**401** | Unauthorized |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

