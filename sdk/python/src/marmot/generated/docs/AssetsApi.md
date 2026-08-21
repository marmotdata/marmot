# marmot.generated.AssetsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**delete_assets_id**](AssetsApi.md#delete_assets_id) | **DELETE** /api/v1/assets/{id} | Delete an asset
[**delete_assets_tags_id**](AssetsApi.md#delete_assets_tags_id) | **DELETE** /api/v1/assets/tags/{id} | Remove tag from asset
[**delete_assets_terms_id**](AssetsApi.md#delete_assets_terms_id) | **DELETE** /api/v1/assets/terms/{id} | Remove glossary term from asset
[**get_assets_by_glossary_term_term_id**](AssetsApi.md#get_assets_by_glossary_term_term_id) | **GET** /api/v1/assets/by-glossary-term/{term_id} | Get assets by glossary term
[**get_assets_documentation_mrn**](AssetsApi.md#get_assets_documentation_mrn) | **GET** /api/v1/assets/documentation/{mrn} | Get asset documentation
[**get_assets_id**](AssetsApi.md#get_assets_id) | **GET** /api/v1/assets/{id} | Get an asset by ID
[**get_assets_id_run_history**](AssetsApi.md#get_assets_id_run_history) | **GET** /api/v1/assets/run-history/{id} | Get asset run history
[**get_assets_id_run_history_histogram**](AssetsApi.md#get_assets_id_run_history_histogram) | **GET** /api/v1/assets/run-history-histogram/{id} | Get asset run history histogram
[**get_assets_lookup_type_service_name**](AssetsApi.md#get_assets_lookup_type_service_name) | **GET** /api/v1/assets/lookup/{type}/{service}/{name} | Lookup asset by type, service, and name
[**get_assets_match_pattern**](AssetsApi.md#get_assets_match_pattern) | **GET** /api/v1/assets/match-pattern/ | Match asset pattern
[**get_assets_my_assets**](AssetsApi.md#get_assets_my_assets) | **GET** /api/v1/assets/my-assets | Get user&#39;s assets
[**get_assets_preview_id**](AssetsApi.md#get_assets_preview_id) | **GET** /api/v1/assets/preview/{id} | Get preview data for an asset
[**get_assets_qualified_name_qualified_name**](AssetsApi.md#get_assets_qualified_name_qualified_name) | **GET** /api/v1/assets/qualified-name/{name} | Get an asset by qualified name
[**get_assets_search**](AssetsApi.md#get_assets_search) | **GET** /api/v1/assets/search | Search assets
[**get_assets_suggestions_metadata_fields**](AssetsApi.md#get_assets_suggestions_metadata_fields) | **GET** /api/v1/assets/suggestions/metadata/fields | Get metadata field suggestions
[**get_assets_suggestions_metadata_values**](AssetsApi.md#get_assets_suggestions_metadata_values) | **GET** /api/v1/assets/suggestions/metadata/values | Get metadata value suggestions
[**get_assets_suggestions_tags**](AssetsApi.md#get_assets_suggestions_tags) | **GET** /api/v1/assets/suggestions/tags | Get tag suggestions
[**get_assets_summary**](AssetsApi.md#get_assets_summary) | **GET** /api/v1/assets/summary | Get asset summary
[**get_assets_terms_id**](AssetsApi.md#get_assets_terms_id) | **GET** /api/v1/assets/terms/{id} | Get asset&#39;s glossary terms
[**post_assets**](AssetsApi.md#post_assets) | **POST** /api/v1/assets/ | Create a new asset
[**post_assets_documentation**](AssetsApi.md#post_assets_documentation) | **POST** /api/v1/assets/documentation/ | Create asset documentation
[**post_assets_documentation_batch**](AssetsApi.md#post_assets_documentation_batch) | **POST** /api/v1/assets/documentation/batch | Batch create documentation
[**post_assets_tags_id**](AssetsApi.md#post_assets_tags_id) | **POST** /api/v1/assets/tags/{id} | Add tag to asset
[**post_assets_terms_id**](AssetsApi.md#post_assets_terms_id) | **POST** /api/v1/assets/terms/{id} | Add glossary terms to asset
[**put_assets_id**](AssetsApi.md#put_assets_id) | **PUT** /api/v1/assets/{id} | Update an asset


# **delete_assets_id**
> delete_assets_id(id)

**Synchronous variant:** `delete_assets_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete an asset

Delete an asset from the system

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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID

    try:
        # Delete an asset
        await api_instance.delete_assets_id(id)
    except Exception as e:
        print("Exception when calling AssetsApi->delete_assets_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Asset ID | 

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
**204** | No Content |  -  |
**404** | Not Found |  -  |
**409** | Conflict |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **delete_assets_tags_id**
> Asset delete_assets_tags_id(id, tag_request)

**Synchronous variant:** `delete_assets_tags_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Remove tag from asset

Remove a tag from an existing asset

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.asset import Asset
from marmot.generated.models.tag_request import TagRequest
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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID
    tag_request = marmot.generated.TagRequest() # TagRequest | Tag to remove

    try:
        # Remove tag from asset
        api_response = await api_instance.delete_assets_tags_id(id, tag_request)
        print("The response of AssetsApi->delete_assets_tags_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->delete_assets_tags_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Asset ID | 
 **tag_request** | [**TagRequest**](TagRequest.md)| Tag to remove | 

### Return type

[**Asset**](Asset.md)

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

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **delete_assets_terms_id**
> List[AssetTerm] delete_assets_terms_id(id, remove_term_request)

**Synchronous variant:** `delete_assets_terms_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Remove glossary term from asset

Remove a glossary term association from an asset

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.asset_term import AssetTerm
from marmot.generated.models.remove_term_request import RemoveTermRequest
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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID
    remove_term_request = marmot.generated.RemoveTermRequest() # RemoveTermRequest | Term ID to remove

    try:
        # Remove glossary term from asset
        api_response = await api_instance.delete_assets_terms_id(id, remove_term_request)
        print("The response of AssetsApi->delete_assets_terms_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->delete_assets_terms_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Asset ID | 
 **remove_term_request** | [**RemoveTermRequest**](RemoveTermRequest.md)| Term ID to remove | 

### Return type

[**List[AssetTerm]**](AssetTerm.md)

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

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **get_assets_by_glossary_term_term_id**
> Dict[str, object] get_assets_by_glossary_term_term_id(term_id, limit=limit, offset=offset)

**Synchronous variant:** `get_assets_by_glossary_term_term_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get assets by glossary term

Retrieve all assets associated with a specific glossary term

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
    api_instance = marmot.generated.AssetsApi(api_client)
    term_id = 'term_id_example' # str | Glossary Term ID
    limit = 20 # int | Maximum number of assets (optional) (default to 20)
    offset = 0 # int | Pagination offset (optional) (default to 0)

    try:
        # Get assets by glossary term
        api_response = await api_instance.get_assets_by_glossary_term_term_id(term_id, limit=limit, offset=offset)
        print("The response of AssetsApi->get_assets_by_glossary_term_term_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->get_assets_by_glossary_term_term_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **term_id** | **str**| Glossary Term ID | 
 **limit** | **int**| Maximum number of assets | [optional] [default to 20]
 **offset** | **int**| Pagination offset | [optional] [default to 0]

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
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **get_assets_documentation_mrn**
> List[Documentation] get_assets_documentation_mrn(mrn)

**Synchronous variant:** `get_assets_documentation_mrn_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get asset documentation

Get documentation for a specific asset

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.documentation import Documentation
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
    api_instance = marmot.generated.AssetsApi(api_client)
    mrn = 'mrn_example' # str | Asset MRN

    try:
        # Get asset documentation
        api_response = await api_instance.get_assets_documentation_mrn(mrn)
        print("The response of AssetsApi->get_assets_documentation_mrn:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->get_assets_documentation_mrn: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **mrn** | **str**| Asset MRN | 

### Return type

[**List[Documentation]**](Documentation.md)

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

# **get_assets_id**
> Asset get_assets_id(id)

**Synchronous variant:** `get_assets_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get an asset by ID

Get detailed information about a specific asset

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.asset import Asset
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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID

    try:
        # Get an asset by ID
        api_response = await api_instance.get_assets_id(id)
        print("The response of AssetsApi->get_assets_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->get_assets_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Asset ID | 

### Return type

[**Asset**](Asset.md)

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

# **get_assets_id_run_history**
> RunHistoryResponse get_assets_id_run_history(id, limit=limit, offset=offset)

**Synchronous variant:** `get_assets_id_run_history_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get asset run history

Get paginated run history for a specific asset

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.run_history_response import RunHistoryResponse
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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID
    limit = 10 # int | Number of items per page (optional) (default to 10)
    offset = 0 # int | Number of items to skip (optional) (default to 0)

    try:
        # Get asset run history
        api_response = await api_instance.get_assets_id_run_history(id, limit=limit, offset=offset)
        print("The response of AssetsApi->get_assets_id_run_history:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->get_assets_id_run_history: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Asset ID | 
 **limit** | **int**| Number of items per page | [optional] [default to 10]
 **offset** | **int**| Number of items to skip | [optional] [default to 0]

### Return type

[**RunHistoryResponse**](RunHistoryResponse.md)

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

# **get_assets_id_run_history_histogram**
> HistogramResponse get_assets_id_run_history_histogram(id, period=period)

**Synchronous variant:** `get_assets_id_run_history_histogram_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get asset run history histogram

Get histogram data for asset run history over specified period

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.histogram_response import HistogramResponse
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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID
    period = '30d' # str | Time period (7d, 30d, 90d) (optional) (default to '30d')

    try:
        # Get asset run history histogram
        api_response = await api_instance.get_assets_id_run_history_histogram(id, period=period)
        print("The response of AssetsApi->get_assets_id_run_history_histogram:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->get_assets_id_run_history_histogram: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Asset ID | 
 **period** | **str**| Time period (7d, 30d, 90d) | [optional] [default to &#39;30d&#39;]

### Return type

[**HistogramResponse**](HistogramResponse.md)

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

# **get_assets_lookup_type_service_name**
> Asset get_assets_lookup_type_service_name(type, service, name)

**Synchronous variant:** `get_assets_lookup_type_service_name_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Lookup asset by type, service, and name

Get an asset by its type, service (provider), and name

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.asset import Asset
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
    api_instance = marmot.generated.AssetsApi(api_client)
    type = 'type_example' # str | Asset type
    service = 'service_example' # str | Service/Provider name
    name = 'name_example' # str | Asset name

    try:
        # Lookup asset by type, service, and name
        api_response = await api_instance.get_assets_lookup_type_service_name(type, service, name)
        print("The response of AssetsApi->get_assets_lookup_type_service_name:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->get_assets_lookup_type_service_name: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **type** | **str**| Asset type | 
 **service** | **str**| Service/Provider name | 
 **name** | **str**| Asset name | 

### Return type

[**Asset**](Asset.md)

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

# **get_assets_match_pattern**
> List[Asset] get_assets_match_pattern(pattern, type)

**Synchronous variant:** `get_assets_match_pattern_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Match asset pattern

Find assets matching a pattern

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.asset import Asset
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
    api_instance = marmot.generated.AssetsApi(api_client)
    pattern = 'pattern_example' # str | Asset pattern to match
    type = 'type_example' # str | Asset type

    try:
        # Match asset pattern
        api_response = await api_instance.get_assets_match_pattern(pattern, type)
        print("The response of AssetsApi->get_assets_match_pattern:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->get_assets_match_pattern: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **pattern** | **str**| Asset pattern to match | 
 **type** | **str**| Asset type | 

### Return type

[**List[Asset]**](Asset.md)

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

# **get_assets_my_assets**
> AssetSearchResponse get_assets_my_assets(limit=limit, offset=offset)

**Synchronous variant:** `get_assets_my_assets_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get user's assets

Get assets owned by the current user or their teams

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.asset_search_response import AssetSearchResponse
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
    api_instance = marmot.generated.AssetsApi(api_client)
    limit = 20 # int | Limit (optional) (default to 20)
    offset = 0 # int | Offset (optional) (default to 0)

    try:
        # Get user's assets
        api_response = await api_instance.get_assets_my_assets(limit=limit, offset=offset)
        print("The response of AssetsApi->get_assets_my_assets:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->get_assets_my_assets: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **limit** | **int**| Limit | [optional] [default to 20]
 **offset** | **int**| Offset | [optional] [default to 0]

### Return type

[**AssetSearchResponse**](AssetSearchResponse.md)

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

# **get_assets_preview_id**
> PreviewResponse get_assets_preview_id(id)

**Synchronous variant:** `get_assets_preview_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get preview data for an asset

Fetches sample data from the asset's data source. Requires assets:preview permission.

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.preview_response import PreviewResponse
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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID

    try:
        # Get preview data for an asset
        api_response = await api_instance.get_assets_preview_id(id)
        print("The response of AssetsApi->get_assets_preview_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->get_assets_preview_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Asset ID | 

### Return type

[**PreviewResponse**](PreviewResponse.md)

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
**403** | Missing assets:preview permission |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |
**501** | Data preview not supported for this asset |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **get_assets_qualified_name_qualified_name**
> Asset get_assets_qualified_name_qualified_name(name)

**Synchronous variant:** `get_assets_qualified_name_qualified_name_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get an asset by qualified name

Get detailed information about a specific asset using its qualified name

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.asset import Asset
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
    api_instance = marmot.generated.AssetsApi(api_client)
    name = 'name_example' # str | Asset qualified name

    try:
        # Get an asset by qualified name
        api_response = await api_instance.get_assets_qualified_name_qualified_name(name)
        print("The response of AssetsApi->get_assets_qualified_name_qualified_name:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->get_assets_qualified_name_qualified_name: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **name** | **str**| Asset qualified name | 

### Return type

[**Asset**](Asset.md)

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

# **get_assets_search**
> AssetSearchResponse get_assets_search(q=q, types=types, services=services, tags=tags, limit=limit, offset=offset, calculate_counts=calculate_counts)

**Synchronous variant:** `get_assets_search_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Search assets

Search for assets using query string and filters

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.asset_search_response import AssetSearchResponse
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
    api_instance = marmot.generated.AssetsApi(api_client)
    q = 'q_example' # str | Search query (optional)
    types = ['types_example'] # List[str] | Filter by asset types (optional)
    services = ['services_example'] # List[str] | Filter by services (optional)
    tags = ['tags_example'] # List[str] | Filter by tags (optional)
    limit = 50 # int | Number of items to return (optional) (default to 50)
    offset = 0 # int | Number of items to skip (optional) (default to 0)
    calculate_counts = False # bool | Calculate filter counts (optional) (default to False)

    try:
        # Search assets
        api_response = await api_instance.get_assets_search(q=q, types=types, services=services, tags=tags, limit=limit, offset=offset, calculate_counts=calculate_counts)
        print("The response of AssetsApi->get_assets_search:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->get_assets_search: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **q** | **str**| Search query | [optional] 
 **types** | [**List[str]**](str.md)| Filter by asset types | [optional] 
 **services** | [**List[str]**](str.md)| Filter by services | [optional] 
 **tags** | [**List[str]**](str.md)| Filter by tags | [optional] 
 **limit** | **int**| Number of items to return | [optional] [default to 50]
 **offset** | **int**| Number of items to skip | [optional] [default to 0]
 **calculate_counts** | **bool**| Calculate filter counts | [optional] [default to False]

### Return type

[**AssetSearchResponse**](AssetSearchResponse.md)

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

# **get_assets_suggestions_metadata_fields**
> List[MetadataFieldSuggestion] get_assets_suggestions_metadata_fields()

**Synchronous variant:** `get_assets_suggestions_metadata_fields_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get metadata field suggestions

Get suggestions for metadata fields and their types

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.metadata_field_suggestion import MetadataFieldSuggestion
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
    api_instance = marmot.generated.AssetsApi(api_client)

    try:
        # Get metadata field suggestions
        api_response = await api_instance.get_assets_suggestions_metadata_fields()
        print("The response of AssetsApi->get_assets_suggestions_metadata_fields:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->get_assets_suggestions_metadata_fields: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**List[MetadataFieldSuggestion]**](MetadataFieldSuggestion.md)

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

# **get_assets_suggestions_metadata_values**
> List[MetadataValueSuggestion] get_assets_suggestions_metadata_values(var_field, prefix=prefix, limit=limit)

**Synchronous variant:** `get_assets_suggestions_metadata_values_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get metadata value suggestions

Get suggestions for values of a specific metadata field

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.metadata_value_suggestion import MetadataValueSuggestion
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
    api_instance = marmot.generated.AssetsApi(api_client)
    var_field = 'var_field_example' # str | Metadata field name
    prefix = 'prefix_example' # str | Value prefix to filter by (optional)
    limit = 10 # int | Maximum number of suggestions (optional) (default to 10)

    try:
        # Get metadata value suggestions
        api_response = await api_instance.get_assets_suggestions_metadata_values(var_field, prefix=prefix, limit=limit)
        print("The response of AssetsApi->get_assets_suggestions_metadata_values:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->get_assets_suggestions_metadata_values: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **var_field** | **str**| Metadata field name | 
 **prefix** | **str**| Value prefix to filter by | [optional] 
 **limit** | **int**| Maximum number of suggestions | [optional] [default to 10]

### Return type

[**List[MetadataValueSuggestion]**](MetadataValueSuggestion.md)

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

# **get_assets_suggestions_tags**
> List[str] get_assets_suggestions_tags(prefix=prefix, limit=limit)

**Synchronous variant:** `get_assets_suggestions_tags_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get tag suggestions

Get suggestions for asset tags

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
    api_instance = marmot.generated.AssetsApi(api_client)
    prefix = 'prefix_example' # str | Tag prefix to filter by (optional)
    limit = 10 # int | Maximum number of suggestions (optional) (default to 10)

    try:
        # Get tag suggestions
        api_response = await api_instance.get_assets_suggestions_tags(prefix=prefix, limit=limit)
        print("The response of AssetsApi->get_assets_suggestions_tags:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->get_assets_suggestions_tags: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **prefix** | **str**| Tag prefix to filter by | [optional] 
 **limit** | **int**| Maximum number of suggestions | [optional] [default to 10]

### Return type

**List[str]**

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

# **get_assets_summary**
> AssetSummaryResponse get_assets_summary()

**Synchronous variant:** `get_assets_summary_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get asset summary

Get the total count of assets by type

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.asset_summary_response import AssetSummaryResponse
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
    api_instance = marmot.generated.AssetsApi(api_client)

    try:
        # Get asset summary
        api_response = await api_instance.get_assets_summary()
        print("The response of AssetsApi->get_assets_summary:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->get_assets_summary: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**AssetSummaryResponse**](AssetSummaryResponse.md)

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

# **get_assets_terms_id**
> List[AssetTerm] get_assets_terms_id(id)

**Synchronous variant:** `get_assets_terms_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get asset's glossary terms

Retrieve all glossary terms associated with an asset

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.asset_term import AssetTerm
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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID

    try:
        # Get asset's glossary terms
        api_response = await api_instance.get_assets_terms_id(id)
        print("The response of AssetsApi->get_assets_terms_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->get_assets_terms_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Asset ID | 

### Return type

[**List[AssetTerm]**](AssetTerm.md)

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

# **post_assets**
> Asset post_assets(create_asset_request)

**Synchronous variant:** `post_assets_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Create a new asset

Create a new asset in the system

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.asset import Asset
from marmot.generated.models.create_asset_request import CreateAssetRequest
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
    api_instance = marmot.generated.AssetsApi(api_client)
    create_asset_request = marmot.generated.CreateAssetRequest() # CreateAssetRequest | Asset creation request

    try:
        # Create a new asset
        api_response = await api_instance.post_assets(create_asset_request)
        print("The response of AssetsApi->post_assets:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->post_assets: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **create_asset_request** | [**CreateAssetRequest**](CreateAssetRequest.md)| Asset creation request | 

### Return type

[**Asset**](Asset.md)

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
**409** | Conflict |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **post_assets_documentation**
> Documentation post_assets_documentation(documentation_create_request)

**Synchronous variant:** `post_assets_documentation_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Create asset documentation

Create or update documentation for an asset

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.documentation import Documentation
from marmot.generated.models.documentation_create_request import DocumentationCreateRequest
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
    api_instance = marmot.generated.AssetsApi(api_client)
    documentation_create_request = marmot.generated.DocumentationCreateRequest() # DocumentationCreateRequest | Documentation creation request

    try:
        # Create asset documentation
        api_response = await api_instance.post_assets_documentation(documentation_create_request)
        print("The response of AssetsApi->post_assets_documentation:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->post_assets_documentation: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **documentation_create_request** | [**DocumentationCreateRequest**](DocumentationCreateRequest.md)| Documentation creation request | 

### Return type

[**Documentation**](Documentation.md)

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

# **post_assets_documentation_batch**
> BatchDocumentationResponse post_assets_documentation_batch(batch_documentation_request)

**Synchronous variant:** `post_assets_documentation_batch_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Batch create documentation

Create or update documentation for multiple assets

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.batch_documentation_request import BatchDocumentationRequest
from marmot.generated.models.batch_documentation_response import BatchDocumentationResponse
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
    api_instance = marmot.generated.AssetsApi(api_client)
    batch_documentation_request = marmot.generated.BatchDocumentationRequest() # BatchDocumentationRequest | Batch documentation request

    try:
        # Batch create documentation
        api_response = await api_instance.post_assets_documentation_batch(batch_documentation_request)
        print("The response of AssetsApi->post_assets_documentation_batch:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->post_assets_documentation_batch: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **batch_documentation_request** | [**BatchDocumentationRequest**](BatchDocumentationRequest.md)| Batch documentation request | 

### Return type

[**BatchDocumentationResponse**](BatchDocumentationResponse.md)

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

# **post_assets_tags_id**
> Asset post_assets_tags_id(id, tag_request)

**Synchronous variant:** `post_assets_tags_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Add tag to asset

Add a new tag to an existing asset

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.asset import Asset
from marmot.generated.models.tag_request import TagRequest
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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID
    tag_request = marmot.generated.TagRequest() # TagRequest | Tag to add

    try:
        # Add tag to asset
        api_response = await api_instance.post_assets_tags_id(id, tag_request)
        print("The response of AssetsApi->post_assets_tags_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->post_assets_tags_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Asset ID | 
 **tag_request** | [**TagRequest**](TagRequest.md)| Tag to add | 

### Return type

[**Asset**](Asset.md)

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

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **post_assets_terms_id**
> List[AssetTerm] post_assets_terms_id(id, add_terms_request)

**Synchronous variant:** `post_assets_terms_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Add glossary terms to asset

Associate one or more glossary terms with an asset

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.add_terms_request import AddTermsRequest
from marmot.generated.models.asset_term import AssetTerm
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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID
    add_terms_request = marmot.generated.AddTermsRequest() # AddTermsRequest | Term IDs to add

    try:
        # Add glossary terms to asset
        api_response = await api_instance.post_assets_terms_id(id, add_terms_request)
        print("The response of AssetsApi->post_assets_terms_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->post_assets_terms_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Asset ID | 
 **add_terms_request** | [**AddTermsRequest**](AddTermsRequest.md)| Term IDs to add | 

### Return type

[**List[AssetTerm]**](AssetTerm.md)

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

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **put_assets_id**
> Asset put_assets_id(id, update_asset_request)

**Synchronous variant:** `put_assets_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Update an asset

Update an existing asset's information

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.asset import Asset
from marmot.generated.models.update_asset_request import UpdateAssetRequest
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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID
    update_asset_request = marmot.generated.UpdateAssetRequest() # UpdateAssetRequest | Asset update request

    try:
        # Update an asset
        api_response = await api_instance.put_assets_id(id, update_asset_request)
        print("The response of AssetsApi->put_assets_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->put_assets_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Asset ID | 
 **update_asset_request** | [**UpdateAssetRequest**](UpdateAssetRequest.md)| Asset update request | 

### Return type

[**Asset**](Asset.md)

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

