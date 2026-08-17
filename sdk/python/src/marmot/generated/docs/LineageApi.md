# marmot.generated.LineageApi

All URIs are relative to */api/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**api_v1_lineage_post**](LineageApi.md#api_v1_lineage_post) | **POST** /api/v1/lineage | Ingest OpenLineage event
[**lineage_assets_id_get**](LineageApi.md#lineage_assets_id_get) | **GET** /lineage/assets/{id} | Get asset lineage
[**lineage_batch_post**](LineageApi.md#lineage_batch_post) | **POST** /lineage/batch | Batch create lineage edges
[**lineage_direct_id_delete**](LineageApi.md#lineage_direct_id_delete) | **DELETE** /lineage/direct/{id} | Delete direct lineage
[**lineage_direct_id_get**](LineageApi.md#lineage_direct_id_get) | **GET** /lineage/direct/{id} | Get direct lineage by ID
[**lineage_direct_post**](LineageApi.md#lineage_direct_post) | **POST** /lineage/direct | Create direct lineage


# **api_v1_lineage_post**
> api_v1_lineage_post(run_event)

**Synchronous variant:** `api_v1_lineage_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Ingest OpenLineage event

Process OpenLineage run events and update assets/lineage accordingly

### Example


```python
import marmot.generated
from marmot.generated.models.run_event import RunEvent
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
    api_instance = marmot.generated.LineageApi(api_client)
    run_event = marmot.generated.RunEvent() # RunEvent | OpenLineage run event

    try:
        # Ingest OpenLineage event
        await api_instance.api_v1_lineage_post(run_event)
    except Exception as e:
        print("Exception when calling LineageApi->api_v1_lineage_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **run_event** | [**RunEvent**](RunEvent.md)| OpenLineage run event | 

### Return type

void (empty response body)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | Event processed successfully |  -  |
**400** | Bad Request |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **lineage_assets_id_get**
> LineageResponse lineage_assets_id_get(id, limit=limit, direction=direction, exclude_types=exclude_types)

**Synchronous variant:** `lineage_assets_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get asset lineage

Get upstream and downstream lineage for a specific asset

### Example


```python
import marmot.generated
from marmot.generated.models.lineage_response import LineageResponse
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
    api_instance = marmot.generated.LineageApi(api_client)
    id = UUID('38400000-8cf0-11bd-b23e-10b96e4ef00d') # UUID | Asset ID
    limit = 10 # int | Maximum depth of lineage graph (optional) (default to 10)
    direction = 'both' # str | Direction of lineage (upstream, downstream, or both) (optional) (default to 'both')
    exclude_types = 'exclude_types_example' # str | Comma separated edge types to leave out, for example CONTAINS to see data flow without structure (optional)

    try:
        # Get asset lineage
        api_response = await api_instance.lineage_assets_id_get(id, limit=limit, direction=direction, exclude_types=exclude_types)
        print("The response of LineageApi->lineage_assets_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling LineageApi->lineage_assets_id_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **UUID**| Asset ID | 
 **limit** | **int**| Maximum depth of lineage graph | [optional] [default to 10]
 **direction** | **str**| Direction of lineage (upstream, downstream, or both) | [optional] [default to &#39;both&#39;]
 **exclude_types** | **str**| Comma separated edge types to leave out, for example CONTAINS to see data flow without structure | [optional] 

### Return type

[**LineageResponse**](LineageResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**400** | Bad Request |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **lineage_batch_post**
> List[BatchLineageResult] lineage_batch_post(lineage_edge)

**Synchronous variant:** `lineage_batch_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Batch create lineage edges

Create lineage edges in batch

### Example


```python
import marmot.generated
from marmot.generated.models.batch_lineage_result import BatchLineageResult
from marmot.generated.models.lineage_edge import LineageEdge
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
    api_instance = marmot.generated.LineageApi(api_client)
    lineage_edge = [marmot.generated.LineageEdge()] # List[LineageEdge] | Array of lineage edges to create

    try:
        # Batch create lineage edges
        api_response = await api_instance.lineage_batch_post(lineage_edge)
        print("The response of LineageApi->lineage_batch_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling LineageApi->lineage_batch_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **lineage_edge** | [**List[LineageEdge]**](LineageEdge.md)| Array of lineage edges to create | 

### Return type

[**List[BatchLineageResult]**](BatchLineageResult.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **lineage_direct_id_delete**
> lineage_direct_id_delete(id)

**Synchronous variant:** `lineage_direct_id_delete_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete direct lineage

Delete a direct lineage connection by its ID

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
    api_instance = marmot.generated.LineageApi(api_client)
    id = UUID('38400000-8cf0-11bd-b23e-10b96e4ef00d') # UUID | Edge ID

    try:
        # Delete direct lineage
        await api_instance.lineage_direct_id_delete(id)
    except Exception as e:
        print("Exception when calling LineageApi->lineage_direct_id_delete: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **UUID**| Edge ID | 

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
**200** | OK |  -  |
**400** | Bad Request |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **lineage_direct_id_get**
> LineageEdge lineage_direct_id_get(id)

**Synchronous variant:** `lineage_direct_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get direct lineage by ID

Get a specific direct lineage connection by its ID

### Example


```python
import marmot.generated
from marmot.generated.models.lineage_edge import LineageEdge
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
    api_instance = marmot.generated.LineageApi(api_client)
    id = UUID('38400000-8cf0-11bd-b23e-10b96e4ef00d') # UUID | Edge ID

    try:
        # Get direct lineage by ID
        api_response = await api_instance.lineage_direct_id_get(id)
        print("The response of LineageApi->lineage_direct_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling LineageApi->lineage_direct_id_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **UUID**| Edge ID | 

### Return type

[**LineageEdge**](LineageEdge.md)

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
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **lineage_direct_post**
> LineageEdge lineage_direct_post(lineage_edge)

**Synchronous variant:** `lineage_direct_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Create direct lineage

Create a direct lineage connection between two assets and returns the created edge

### Example


```python
import marmot.generated
from marmot.generated.models.lineage_edge import LineageEdge
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
    api_instance = marmot.generated.LineageApi(api_client)
    lineage_edge = marmot.generated.LineageEdge() # LineageEdge | Lineage edge to create

    try:
        # Create direct lineage
        api_response = await api_instance.lineage_direct_post(lineage_edge)
        print("The response of LineageApi->lineage_direct_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling LineageApi->lineage_direct_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **lineage_edge** | [**LineageEdge**](LineageEdge.md)| Lineage edge to create | 

### Return type

[**LineageEdge**](LineageEdge.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**400** | Bad Request |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

