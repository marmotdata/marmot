# marmot.generated.MetricsApi

All URIs are relative to */api/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**metrics_assets_by_owner_get**](MetricsApi.md#metrics_assets_by_owner_get) | **GET** /metrics/assets/by-owner | Get assets by owner
[**metrics_assets_by_provider_get**](MetricsApi.md#metrics_assets_by_provider_get) | **GET** /metrics/assets/by-provider | Get assets by provider
[**metrics_assets_by_type_get**](MetricsApi.md#metrics_assets_by_type_get) | **GET** /metrics/assets/by-type | Get assets by type
[**metrics_assets_total_get**](MetricsApi.md#metrics_assets_total_get) | **GET** /metrics/assets/total | Get total assets count
[**metrics_assets_with_schemas_get**](MetricsApi.md#metrics_assets_with_schemas_get) | **GET** /metrics/assets/with-schemas | Get assets with schemas count
[**metrics_get**](MetricsApi.md#metrics_get) | **GET** /metrics | Get metrics for UI
[**metrics_top_assets_get**](MetricsApi.md#metrics_top_assets_get) | **GET** /metrics/top-assets | Get top viewed assets
[**metrics_top_queries_get**](MetricsApi.md#metrics_top_queries_get) | **GET** /metrics/top-queries | Get top search queries


# **metrics_assets_by_owner_get**
> AssetsByOwnerResponse metrics_assets_by_owner_get()

**Synchronous variant:** `metrics_assets_by_owner_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.MetricsApi(api_client)

    try:
        # Get assets by owner
        api_response = await api_instance.metrics_assets_by_owner_get()
        print("The response of MetricsApi->metrics_assets_by_owner_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling MetricsApi->metrics_assets_by_owner_get: %s\n" % e)
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

# **metrics_assets_by_provider_get**
> AssetsByProviderResponse metrics_assets_by_provider_get()

**Synchronous variant:** `metrics_assets_by_provider_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.MetricsApi(api_client)

    try:
        # Get assets by provider
        api_response = await api_instance.metrics_assets_by_provider_get()
        print("The response of MetricsApi->metrics_assets_by_provider_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling MetricsApi->metrics_assets_by_provider_get: %s\n" % e)
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

# **metrics_assets_by_type_get**
> AssetsByTypeResponse metrics_assets_by_type_get()

**Synchronous variant:** `metrics_assets_by_type_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.MetricsApi(api_client)

    try:
        # Get assets by type
        api_response = await api_instance.metrics_assets_by_type_get()
        print("The response of MetricsApi->metrics_assets_by_type_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling MetricsApi->metrics_assets_by_type_get: %s\n" % e)
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

# **metrics_assets_total_get**
> TotalAssetsResponse metrics_assets_total_get()

**Synchronous variant:** `metrics_assets_total_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.MetricsApi(api_client)

    try:
        # Get total assets count
        api_response = await api_instance.metrics_assets_total_get()
        print("The response of MetricsApi->metrics_assets_total_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling MetricsApi->metrics_assets_total_get: %s\n" % e)
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

# **metrics_assets_with_schemas_get**
> AssetsWithSchemasResponse metrics_assets_with_schemas_get()

**Synchronous variant:** `metrics_assets_with_schemas_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.MetricsApi(api_client)

    try:
        # Get assets with schemas count
        api_response = await api_instance.metrics_assets_with_schemas_get()
        print("The response of MetricsApi->metrics_assets_with_schemas_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling MetricsApi->metrics_assets_with_schemas_get: %s\n" % e)
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

# **metrics_get**
> GetMetricsResponse metrics_get(start, end, metric_names=metric_names, aggregation=aggregation, bucket_size=bucket_size)

**Synchronous variant:** `metrics_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.MetricsApi(api_client)
    start = 'start_example' # str | Start time (ISO 8601)
    end = 'end_example' # str | End time (ISO 8601)
    metric_names = ['metric_names_example'] # List[str] | Filter by metric names (optional)
    aggregation = 'avg' # str | Aggregation type (optional) (default to 'avg')
    bucket_size = 'bucket_size_example' # str | Time bucket size (optional)

    try:
        # Get metrics for UI
        api_response = await api_instance.metrics_get(start, end, metric_names=metric_names, aggregation=aggregation, bucket_size=bucket_size)
        print("The response of MetricsApi->metrics_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling MetricsApi->metrics_get: %s\n" % e)
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

# **metrics_top_assets_get**
> List[AssetCount] metrics_top_assets_get(start, end, limit=limit)

**Synchronous variant:** `metrics_top_assets_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.MetricsApi(api_client)
    start = 'start_example' # str | Start time (ISO 8601)
    end = 'end_example' # str | End time (ISO 8601)
    limit = 10 # int | Number of results (optional) (default to 10)

    try:
        # Get top viewed assets
        api_response = await api_instance.metrics_top_assets_get(start, end, limit=limit)
        print("The response of MetricsApi->metrics_top_assets_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling MetricsApi->metrics_top_assets_get: %s\n" % e)
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

# **metrics_top_queries_get**
> List[QueryCount] metrics_top_queries_get(start, end, limit=limit)

**Synchronous variant:** `metrics_top_queries_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.MetricsApi(api_client)
    start = 'start_example' # str | Start time (ISO 8601)
    end = 'end_example' # str | End time (ISO 8601)
    limit = 10 # int | Number of results (optional) (default to 10)

    try:
        # Get top search queries
        api_response = await api_instance.metrics_top_queries_get(start, end, limit=limit)
        print("The response of MetricsApi->metrics_top_queries_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling MetricsApi->metrics_top_queries_get: %s\n" % e)
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

