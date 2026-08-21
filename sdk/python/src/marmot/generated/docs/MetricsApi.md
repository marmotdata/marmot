# marmot.generated.MetricsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**get_metrics**](MetricsApi.md#get_metrics) | **GET** /api/v1/metrics | Get metrics for UI
[**get_metrics_assets_by_owner**](MetricsApi.md#get_metrics_assets_by_owner) | **GET** /api/v1/metrics/assets/by-owner | Get assets by owner
[**get_metrics_assets_by_provider**](MetricsApi.md#get_metrics_assets_by_provider) | **GET** /api/v1/metrics/assets/by-provider | Get assets by provider
[**get_metrics_assets_by_type**](MetricsApi.md#get_metrics_assets_by_type) | **GET** /api/v1/metrics/assets/by-type | Get assets by type
[**get_metrics_assets_total**](MetricsApi.md#get_metrics_assets_total) | **GET** /api/v1/metrics/assets/total | Get total assets count
[**get_metrics_assets_with_schemas**](MetricsApi.md#get_metrics_assets_with_schemas) | **GET** /api/v1/metrics/assets/with-schemas | Get assets with schemas count
[**get_metrics_top_assets**](MetricsApi.md#get_metrics_top_assets) | **GET** /api/v1/metrics/top-assets | Get top viewed assets
[**get_metrics_top_queries**](MetricsApi.md#get_metrics_top_queries) | **GET** /api/v1/metrics/top-queries | Get top search queries


# **get_metrics**
> GetMetricsResponse get_metrics(start, end, metric_names=metric_names, aggregation=aggregation, bucket_size=bucket_size)

**Synchronous variant:** `get_metrics_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get metrics for UI

Get aggregated metrics for dashboard display

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.get_metrics_response import GetMetricsResponse
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
    api_instance = marmot.generated.MetricsApi(api_client)
    start = 'start_example' # str | Start time (ISO 8601)
    end = 'end_example' # str | End time (ISO 8601)
    metric_names = ['metric_names_example'] # List[str] | Filter by metric names (optional)
    aggregation = 'avg' # str | Aggregation type (optional) (default to 'avg')
    bucket_size = 'bucket_size_example' # str | Time bucket size (optional)

    try:
        # Get metrics for UI
        api_response = await api_instance.get_metrics(start, end, metric_names=metric_names, aggregation=aggregation, bucket_size=bucket_size)
        print("The response of MetricsApi->get_metrics:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling MetricsApi->get_metrics: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **start** | **str**| Start time (ISO 8601) | 
 **end** | **str**| End time (ISO 8601) | 
 **metric_names** | [**List[str]**](str.md)| Filter by metric names | [optional] 
 **aggregation** | **str**| Aggregation type | [optional] [default to &#39;avg&#39;]
 **bucket_size** | **str**| Time bucket size | [optional] 

### Return type

[**GetMetricsResponse**](GetMetricsResponse.md)

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
**401** | Unauthorized |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **get_metrics_assets_by_owner**
> AssetsByOwnerResponse get_metrics_assets_by_owner()

**Synchronous variant:** `get_metrics_assets_by_owner_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get assets by owner

Get asset counts grouped by owner

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.assets_by_owner_response import AssetsByOwnerResponse
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
    api_instance = marmot.generated.MetricsApi(api_client)

    try:
        # Get assets by owner
        api_response = await api_instance.get_metrics_assets_by_owner()
        print("The response of MetricsApi->get_metrics_assets_by_owner:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling MetricsApi->get_metrics_assets_by_owner: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**AssetsByOwnerResponse**](AssetsByOwnerResponse.md)

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

# **get_metrics_assets_by_provider**
> AssetsByProviderResponse get_metrics_assets_by_provider()

**Synchronous variant:** `get_metrics_assets_by_provider_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get assets by provider

Get asset counts grouped by provider

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.assets_by_provider_response import AssetsByProviderResponse
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
    api_instance = marmot.generated.MetricsApi(api_client)

    try:
        # Get assets by provider
        api_response = await api_instance.get_metrics_assets_by_provider()
        print("The response of MetricsApi->get_metrics_assets_by_provider:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling MetricsApi->get_metrics_assets_by_provider: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**AssetsByProviderResponse**](AssetsByProviderResponse.md)

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

# **get_metrics_assets_by_type**
> AssetsByTypeResponse get_metrics_assets_by_type()

**Synchronous variant:** `get_metrics_assets_by_type_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get assets by type

Get asset counts grouped by type

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.assets_by_type_response import AssetsByTypeResponse
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
    api_instance = marmot.generated.MetricsApi(api_client)

    try:
        # Get assets by type
        api_response = await api_instance.get_metrics_assets_by_type()
        print("The response of MetricsApi->get_metrics_assets_by_type:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling MetricsApi->get_metrics_assets_by_type: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**AssetsByTypeResponse**](AssetsByTypeResponse.md)

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

# **get_metrics_assets_total**
> TotalAssetsResponse get_metrics_assets_total()

**Synchronous variant:** `get_metrics_assets_total_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get total assets count

Get the total number of assets

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.total_assets_response import TotalAssetsResponse
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
    api_instance = marmot.generated.MetricsApi(api_client)

    try:
        # Get total assets count
        api_response = await api_instance.get_metrics_assets_total()
        print("The response of MetricsApi->get_metrics_assets_total:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling MetricsApi->get_metrics_assets_total: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**TotalAssetsResponse**](TotalAssetsResponse.md)

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

# **get_metrics_assets_with_schemas**
> AssetsWithSchemasResponse get_metrics_assets_with_schemas()

**Synchronous variant:** `get_metrics_assets_with_schemas_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get assets with schemas count

Get the count of assets that have schemas defined

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.assets_with_schemas_response import AssetsWithSchemasResponse
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
    api_instance = marmot.generated.MetricsApi(api_client)

    try:
        # Get assets with schemas count
        api_response = await api_instance.get_metrics_assets_with_schemas()
        print("The response of MetricsApi->get_metrics_assets_with_schemas:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling MetricsApi->get_metrics_assets_with_schemas: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**AssetsWithSchemasResponse**](AssetsWithSchemasResponse.md)

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

# **get_metrics_top_assets**
> List[AssetCount] get_metrics_top_assets(start, end, limit=limit)

**Synchronous variant:** `get_metrics_top_assets_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get top viewed assets

Get the most viewed assets

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.asset_count import AssetCount
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
    api_instance = marmot.generated.MetricsApi(api_client)
    start = 'start_example' # str | Start time (ISO 8601)
    end = 'end_example' # str | End time (ISO 8601)
    limit = 10 # int | Number of results (optional) (default to 10)

    try:
        # Get top viewed assets
        api_response = await api_instance.get_metrics_top_assets(start, end, limit=limit)
        print("The response of MetricsApi->get_metrics_top_assets:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling MetricsApi->get_metrics_top_assets: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **start** | **str**| Start time (ISO 8601) | 
 **end** | **str**| End time (ISO 8601) | 
 **limit** | **int**| Number of results | [optional] [default to 10]

### Return type

[**List[AssetCount]**](AssetCount.md)

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

# **get_metrics_top_queries**
> List[QueryCount] get_metrics_top_queries(start, end, limit=limit)

**Synchronous variant:** `get_metrics_top_queries_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get top search queries

Get the most popular search queries

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.query_count import QueryCount
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
    api_instance = marmot.generated.MetricsApi(api_client)
    start = 'start_example' # str | Start time (ISO 8601)
    end = 'end_example' # str | End time (ISO 8601)
    limit = 10 # int | Number of results (optional) (default to 10)

    try:
        # Get top search queries
        api_response = await api_instance.get_metrics_top_queries(start, end, limit=limit)
        print("The response of MetricsApi->get_metrics_top_queries:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling MetricsApi->get_metrics_top_queries: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **start** | **str**| Start time (ISO 8601) | 
 **end** | **str**| End time (ISO 8601) | 
 **limit** | **int**| Number of results | [optional] [default to 10]

### Return type

[**List[QueryCount]**](QueryCount.md)

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

