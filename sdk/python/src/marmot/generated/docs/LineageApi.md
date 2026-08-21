# marmot.generated.LineageApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**delete_lineage_direct_id**](LineageApi.md#delete_lineage_direct_id) | **DELETE** /api/v1/lineage/direct/{id} | Delete direct lineage
[**get_lineage_assets_id**](LineageApi.md#get_lineage_assets_id) | **GET** /api/v1/lineage/assets/{id} | Get asset lineage
[**get_lineage_direct_id**](LineageApi.md#get_lineage_direct_id) | **GET** /api/v1/lineage/direct/{id} | Get direct lineage by ID
[**post_lineage**](LineageApi.md#post_lineage) | **POST** /api/v1/lineage | Ingest OpenLineage event
[**post_lineage_batch**](LineageApi.md#post_lineage_batch) | **POST** /api/v1/lineage/batch | Batch create lineage edges
[**post_lineage_direct**](LineageApi.md#post_lineage_direct) | **POST** /api/v1/lineage/direct | Create direct lineage


# **delete_lineage_direct_id**
> delete_lineage_direct_id(id)

**Synchronous variant:** `delete_lineage_direct_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete direct lineage

Delete a direct lineage connection by its ID

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
    api_instance = marmot.generated.LineageApi(api_client)
    id = UUID('38400000-8cf0-11bd-b23e-10b96e4ef00d') # UUID | Edge ID

    try:
        # Delete direct lineage
        await api_instance.delete_lineage_direct_id(id)
    except Exception as e:
        print("Exception when calling LineageApi->delete_lineage_direct_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **UUID**| Edge ID | 

### Return type

void (empty response body)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

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

# **get_lineage_assets_id**
> LineageResponse get_lineage_assets_id(id, limit=limit, direction=direction, exclude_types=exclude_types)

**Synchronous variant:** `get_lineage_assets_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get asset lineage

Get upstream and downstream lineage for a specific asset

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.lineage_response import LineageResponse
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
    api_instance = marmot.generated.LineageApi(api_client)
    id = UUID('38400000-8cf0-11bd-b23e-10b96e4ef00d') # UUID | Asset ID
    limit = 10 # int | Maximum depth of lineage graph (optional) (default to 10)
    direction = 'both' # str | Direction of lineage (upstream, downstream, or both) (optional) (default to 'both')
    exclude_types = 'exclude_types_example' # str | Comma separated edge types to leave out, for example CONTAINS to see data flow without structure (optional)

    try:
        # Get asset lineage
        api_response = await api_instance.get_lineage_assets_id(id, limit=limit, direction=direction, exclude_types=exclude_types)
        print("The response of LineageApi->get_lineage_assets_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling LineageApi->get_lineage_assets_id: %s\n" % e)
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

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

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

# **get_lineage_direct_id**
> LineageEdge get_lineage_direct_id(id)

**Synchronous variant:** `get_lineage_direct_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get direct lineage by ID

Get a specific direct lineage connection by its ID

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.lineage_edge import LineageEdge
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
    api_instance = marmot.generated.LineageApi(api_client)
    id = UUID('38400000-8cf0-11bd-b23e-10b96e4ef00d') # UUID | Edge ID

    try:
        # Get direct lineage by ID
        api_response = await api_instance.get_lineage_direct_id(id)
        print("The response of LineageApi->get_lineage_direct_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling LineageApi->get_lineage_direct_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **UUID**| Edge ID | 

### Return type

[**LineageEdge**](LineageEdge.md)

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

# **post_lineage**
> post_lineage(run_event)

**Synchronous variant:** `post_lineage_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Ingest OpenLineage event

Process OpenLineage run events and update assets/lineage accordingly

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.run_event import RunEvent
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
    api_instance = marmot.generated.LineageApi(api_client)
    run_event = marmot.generated.RunEvent() # RunEvent | OpenLineage run event

    try:
        # Ingest OpenLineage event
        await api_instance.post_lineage(run_event)
    except Exception as e:
        print("Exception when calling LineageApi->post_lineage: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **run_event** | [**RunEvent**](RunEvent.md)| OpenLineage run event | 

### Return type

void (empty response body)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

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

# **post_lineage_batch**
> List[BatchLineageResult] post_lineage_batch(lineage_edge)

**Synchronous variant:** `post_lineage_batch_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Batch create lineage edges

Create lineage edges in batch

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.batch_lineage_result import BatchLineageResult
from marmot.generated.models.lineage_edge import LineageEdge
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
    api_instance = marmot.generated.LineageApi(api_client)
    lineage_edge = [marmot.generated.LineageEdge()] # List[LineageEdge] | Array of lineage edges to create

    try:
        # Batch create lineage edges
        api_response = await api_instance.post_lineage_batch(lineage_edge)
        print("The response of LineageApi->post_lineage_batch:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling LineageApi->post_lineage_batch: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **lineage_edge** | [**List[LineageEdge]**](LineageEdge.md)| Array of lineage edges to create | 

### Return type

[**List[BatchLineageResult]**](BatchLineageResult.md)

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

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **post_lineage_direct**
> LineageEdge post_lineage_direct(lineage_edge)

**Synchronous variant:** `post_lineage_direct_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Create direct lineage

Create a direct lineage connection between two assets and returns the created edge

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.lineage_edge import LineageEdge
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
    api_instance = marmot.generated.LineageApi(api_client)
    lineage_edge = marmot.generated.LineageEdge() # LineageEdge | Lineage edge to create

    try:
        # Create direct lineage
        api_response = await api_instance.post_lineage_direct(lineage_edge)
        print("The response of LineageApi->post_lineage_direct:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling LineageApi->post_lineage_direct: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **lineage_edge** | [**LineageEdge**](LineageEdge.md)| Lineage edge to create | 

### Return type

[**LineageEdge**](LineageEdge.md)

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
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

