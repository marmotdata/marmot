# marmot.generated.RunsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**get_runs**](RunsApi.md#get_runs) | **GET** /api/v1/runs | List runs
[**get_runs_id**](RunsApi.md#get_runs_id) | **GET** /api/v1/runs/{id} | Get run
[**get_runs_id_entities**](RunsApi.md#get_runs_id_entities) | **GET** /api/v1/runs/{id}/entities | Get run entities
[**post_runs_assets_batch**](RunsApi.md#post_runs_assets_batch) | **POST** /api/v1/runs/assets/batch | Batch create assets
[**post_runs_cleanup**](RunsApi.md#post_runs_cleanup) | **POST** /api/v1/runs/cleanup | Cleanup stale runs
[**post_runs_complete**](RunsApi.md#post_runs_complete) | **POST** /api/v1/runs/complete | Complete run
[**post_runs_start**](RunsApi.md#post_runs_start) | **POST** /api/v1/runs/start | Start run


# **get_runs**
> GetRuns200Response get_runs(pipelines=pipelines, statuses=statuses, limit=limit, offset=offset)

**Synchronous variant:** `get_runs_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List runs

Get paginated list of runs with filtering

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.get_runs200_response import GetRuns200Response
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
    api_instance = marmot.generated.RunsApi(api_client)
    pipelines = 'pipelines_example' # str | Comma-separated list of pipeline names (optional)
    statuses = 'statuses_example' # str | Comma-separated list of statuses (optional)
    limit = 50 # int | Number of results per page (optional) (default to 50)
    offset = 0 # int | Number of results to skip (optional) (default to 0)

    try:
        # List runs
        api_response = await api_instance.get_runs(pipelines=pipelines, statuses=statuses, limit=limit, offset=offset)
        print("The response of RunsApi->get_runs:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling RunsApi->get_runs: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **pipelines** | **str**| Comma-separated list of pipeline names | [optional] 
 **statuses** | **str**| Comma-separated list of statuses | [optional] 
 **limit** | **int**| Number of results per page | [optional] [default to 50]
 **offset** | **int**| Number of results to skip | [optional] [default to 0]

### Return type

[**GetRuns200Response**](GetRuns200Response.md)

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

# **get_runs_id**
> PluginRun get_runs_id(id)

**Synchronous variant:** `get_runs_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.RunsApi(api_client)
    id = 'id_example' # str | Run ID

    try:
        # Get run
        api_response = await api_instance.get_runs_id(id)
        print("The response of RunsApi->get_runs_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling RunsApi->get_runs_id: %s\n" % e)
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

# **get_runs_id_entities**
> RunEntitiesResponse get_runs_id_entities(id, entity_type=entity_type, status=status, limit=limit, offset=offset)

**Synchronous variant:** `get_runs_id_entities_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.RunsApi(api_client)
    id = 'id_example' # str | Run ID
    entity_type = 'entity_type_example' # str | Filter by entity type (asset, lineage, documentation) (optional)
    status = 'status_example' # str | Filter by status (created, updated, deleted, failed) (optional)
    limit = 100 # int | Number of results per page (optional) (default to 100)
    offset = 0 # int | Number of results to skip (optional) (default to 0)

    try:
        # Get run entities
        api_response = await api_instance.get_runs_id_entities(id, entity_type=entity_type, status=status, limit=limit, offset=offset)
        print("The response of RunsApi->get_runs_id_entities:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling RunsApi->get_runs_id_entities: %s\n" % e)
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

# **post_runs_assets_batch**
> BatchCreateResponse post_runs_assets_batch(batch_create_request)

**Synchronous variant:** `post_runs_assets_batch_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.RunsApi(api_client)
    batch_create_request = marmot.generated.BatchCreateRequest() # BatchCreateRequest | Batch create request

    try:
        # Batch create assets
        api_response = await api_instance.post_runs_assets_batch(batch_create_request)
        print("The response of RunsApi->post_runs_assets_batch:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling RunsApi->post_runs_assets_batch: %s\n" % e)
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

# **post_runs_cleanup**
> Dict[str, int] post_runs_cleanup()

**Synchronous variant:** `post_runs_cleanup_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Cleanup stale runs

Mark runs as failed if they've been running too long without updates

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
    api_instance = marmot.generated.RunsApi(api_client)

    try:
        # Cleanup stale runs
        api_response = await api_instance.post_runs_cleanup()
        print("The response of RunsApi->post_runs_cleanup:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling RunsApi->post_runs_cleanup: %s\n" % e)
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

# **post_runs_complete**
> Dict[str, str] post_runs_complete(complete_run_request)

**Synchronous variant:** `post_runs_complete_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.RunsApi(api_client)
    complete_run_request = marmot.generated.CompleteRunRequest() # CompleteRunRequest | Complete run request

    try:
        # Complete run
        api_response = await api_instance.post_runs_complete(complete_run_request)
        print("The response of RunsApi->post_runs_complete:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling RunsApi->post_runs_complete: %s\n" % e)
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

# **post_runs_start**
> PluginRun post_runs_start(start_run_request)

**Synchronous variant:** `post_runs_start_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.RunsApi(api_client)
    start_run_request = marmot.generated.StartRunRequest() # StartRunRequest | Start run request

    try:
        # Start run
        api_response = await api_instance.post_runs_start(start_run_request)
        print("The response of RunsApi->post_runs_start:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling RunsApi->post_runs_start: %s\n" % e)
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

