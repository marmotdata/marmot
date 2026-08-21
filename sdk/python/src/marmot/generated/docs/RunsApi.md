# marmot.generated.RunsApi

All URIs are relative to */api/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**runs_assets_batch_post**](RunsApi.md#runs_assets_batch_post) | **POST** /runs/assets/batch | Batch create assets
[**runs_cleanup_post**](RunsApi.md#runs_cleanup_post) | **POST** /runs/cleanup | Cleanup stale runs
[**runs_complete_post**](RunsApi.md#runs_complete_post) | **POST** /runs/complete | Complete run
[**runs_get**](RunsApi.md#runs_get) | **GET** /runs | List runs
[**runs_id_entities_get**](RunsApi.md#runs_id_entities_get) | **GET** /runs/{id}/entities | Get run entities
[**runs_id_get**](RunsApi.md#runs_id_get) | **GET** /runs/{id} | Get run
[**runs_start_post**](RunsApi.md#runs_start_post) | **POST** /runs/start | Start run


# **runs_assets_batch_post**
> BatchCreateResponse runs_assets_batch_post(batch_create_request)

**Synchronous variant:** `runs_assets_batch_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Batch create assets

Create/update assets within a run

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.batch_create_request import BatchCreateRequest
from marmot.generated.models.batch_create_response import BatchCreateResponse
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
    api_instance = marmot.generated.RunsApi(api_client)
    batch_create_request = marmot.generated.BatchCreateRequest() # BatchCreateRequest | Batch create request

    try:
        # Batch create assets
        api_response = await api_instance.runs_assets_batch_post(batch_create_request)
        print("The response of RunsApi->runs_assets_batch_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling RunsApi->runs_assets_batch_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **batch_create_request** | [**BatchCreateRequest**](BatchCreateRequest.md)| Batch create request | 

### Return type

[**BatchCreateResponse**](BatchCreateResponse.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **runs_cleanup_post**
> Dict[str, int] runs_cleanup_post()

**Synchronous variant:** `runs_cleanup_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Cleanup stale runs

Mark runs as failed if they've been running too long without updates

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
    api_instance = marmot.generated.RunsApi(api_client)

    try:
        # Cleanup stale runs
        api_response = await api_instance.runs_cleanup_post()
        print("The response of RunsApi->runs_cleanup_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling RunsApi->runs_cleanup_post: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

**Dict[str, int]**

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: */*

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **runs_complete_post**
> Dict[str, str] runs_complete_post(complete_run_request)

**Synchronous variant:** `runs_complete_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Complete run

Complete a run with results

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.complete_run_request import CompleteRunRequest
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
    api_instance = marmot.generated.RunsApi(api_client)
    complete_run_request = marmot.generated.CompleteRunRequest() # CompleteRunRequest | Complete run request

    try:
        # Complete run
        api_response = await api_instance.runs_complete_post(complete_run_request)
        print("The response of RunsApi->runs_complete_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling RunsApi->runs_complete_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **complete_run_request** | [**CompleteRunRequest**](CompleteRunRequest.md)| Complete run request | 

### Return type

**Dict[str, str]**

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **runs_get**
> RunsGet200Response runs_get(pipelines=pipelines, statuses=statuses, limit=limit, offset=offset)

**Synchronous variant:** `runs_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List runs

Get paginated list of runs with filtering

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.runs_get200_response import RunsGet200Response
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
    api_instance = marmot.generated.RunsApi(api_client)
    pipelines = 'pipelines_example' # str | Comma-separated list of pipeline names (optional)
    statuses = 'statuses_example' # str | Comma-separated list of statuses (optional)
    limit = 50 # int | Number of results per page (optional) (default to 50)
    offset = 0 # int | Number of results to skip (optional) (default to 0)

    try:
        # List runs
        api_response = await api_instance.runs_get(pipelines=pipelines, statuses=statuses, limit=limit, offset=offset)
        print("The response of RunsApi->runs_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling RunsApi->runs_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **pipelines** | **str**| Comma-separated list of pipeline names | [optional] 
 **statuses** | **str**| Comma-separated list of statuses | [optional] 
 **limit** | **int**| Number of results per page | [optional] [default to 50]
 **offset** | **int**| Number of results to skip | [optional] [default to 0]

### Return type

[**RunsGet200Response**](RunsGet200Response.md)

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

# **runs_id_entities_get**
> RunEntitiesResponse runs_id_entities_get(id, entity_type=entity_type, status=status, limit=limit, offset=offset)

**Synchronous variant:** `runs_id_entities_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get run entities

Get paginated list of entities for a specific run

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.run_entities_response import RunEntitiesResponse
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
    api_instance = marmot.generated.RunsApi(api_client)
    id = 'id_example' # str | Run ID
    entity_type = 'entity_type_example' # str | Filter by entity type (asset, lineage, documentation) (optional)
    status = 'status_example' # str | Filter by status (created, updated, deleted, failed) (optional)
    limit = 100 # int | Number of results per page (optional) (default to 100)
    offset = 0 # int | Number of results to skip (optional) (default to 0)

    try:
        # Get run entities
        api_response = await api_instance.runs_id_entities_get(id, entity_type=entity_type, status=status, limit=limit, offset=offset)
        print("The response of RunsApi->runs_id_entities_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling RunsApi->runs_id_entities_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Run ID | 
 **entity_type** | **str**| Filter by entity type (asset, lineage, documentation) | [optional] 
 **status** | **str**| Filter by status (created, updated, deleted, failed) | [optional] 
 **limit** | **int**| Number of results per page | [optional] [default to 100]
 **offset** | **int**| Number of results to skip | [optional] [default to 0]

### Return type

[**RunEntitiesResponse**](RunEntitiesResponse.md)

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

# **runs_id_get**
> PluginRun runs_id_get(id)

**Synchronous variant:** `runs_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get run

Get a specific run by ID

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.plugin_run import PluginRun
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
    api_instance = marmot.generated.RunsApi(api_client)
    id = 'id_example' # str | Run ID

    try:
        # Get run
        api_response = await api_instance.runs_id_get(id)
        print("The response of RunsApi->runs_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling RunsApi->runs_id_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Run ID | 

### Return type

[**PluginRun**](PluginRun.md)

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

# **runs_start_post**
> PluginRun runs_start_post(start_run_request)

**Synchronous variant:** `runs_start_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Start run

Start a new run for tracking

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.plugin_run import PluginRun
from marmot.generated.models.start_run_request import StartRunRequest
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
    api_instance = marmot.generated.RunsApi(api_client)
    start_run_request = marmot.generated.StartRunRequest() # StartRunRequest | Start run request

    try:
        # Start run
        api_response = await api_instance.runs_start_post(start_run_request)
        print("The response of RunsApi->runs_start_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling RunsApi->runs_start_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **start_run_request** | [**StartRunRequest**](StartRunRequest.md)| Start run request | 

### Return type

[**PluginRun**](PluginRun.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

