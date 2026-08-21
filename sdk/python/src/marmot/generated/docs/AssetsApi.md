# marmot.generated.AssetsApi

All URIs are relative to */api/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**assets_by_glossary_term_term_id_get**](AssetsApi.md#assets_by_glossary_term_term_id_get) | **GET** /assets/by-glossary-term/{term_id} | Get assets by glossary term
[**assets_documentation_batch_post**](AssetsApi.md#assets_documentation_batch_post) | **POST** /assets/documentation/batch | Batch create documentation
[**assets_documentation_mrn_get**](AssetsApi.md#assets_documentation_mrn_get) | **GET** /assets/documentation/{mrn} | Get asset documentation
[**assets_documentation_post**](AssetsApi.md#assets_documentation_post) | **POST** /assets/documentation | Create asset documentation
[**assets_id_delete**](AssetsApi.md#assets_id_delete) | **DELETE** /assets/{id} | Delete an asset
[**assets_id_get**](AssetsApi.md#assets_id_get) | **GET** /assets/{id} | Get an asset by ID
[**assets_id_put**](AssetsApi.md#assets_id_put) | **PUT** /assets/{id} | Update an asset
[**assets_id_run_history_get**](AssetsApi.md#assets_id_run_history_get) | **GET** /assets/{id}/run-history | Get asset run history
[**assets_id_run_history_histogram_get**](AssetsApi.md#assets_id_run_history_histogram_get) | **GET** /assets/{id}/run-history/histogram | Get asset run history histogram
[**assets_lookup_type_service_name_get**](AssetsApi.md#assets_lookup_type_service_name_get) | **GET** /assets/lookup/{type}/{service}/{name} | Lookup asset by type, service, and name
[**assets_match_pattern_get**](AssetsApi.md#assets_match_pattern_get) | **GET** /assets/match-pattern | Match asset pattern
[**assets_my_assets_get**](AssetsApi.md#assets_my_assets_get) | **GET** /assets/my-assets | Get user&#39;s assets
[**assets_post**](AssetsApi.md#assets_post) | **POST** /assets | Create a new asset
[**assets_preview_id_get**](AssetsApi.md#assets_preview_id_get) | **GET** /assets/preview/{id} | Get preview data for an asset
[**assets_qualified_name_qualified_name_get**](AssetsApi.md#assets_qualified_name_qualified_name_get) | **GET** /assets/qualified-name/{qualifiedName} | Get an asset by qualified name
[**assets_search_get**](AssetsApi.md#assets_search_get) | **GET** /assets/search | Search assets
[**assets_suggestions_metadata_fields_get**](AssetsApi.md#assets_suggestions_metadata_fields_get) | **GET** /assets/suggestions/metadata/fields | Get metadata field suggestions
[**assets_suggestions_metadata_values_get**](AssetsApi.md#assets_suggestions_metadata_values_get) | **GET** /assets/suggestions/metadata/values | Get metadata value suggestions
[**assets_suggestions_tags_get**](AssetsApi.md#assets_suggestions_tags_get) | **GET** /assets/suggestions/tags | Get tag suggestions
[**assets_summary_get**](AssetsApi.md#assets_summary_get) | **GET** /assets/summary | Get asset summary
[**assets_tags_id_delete**](AssetsApi.md#assets_tags_id_delete) | **DELETE** /assets/tags/{id} | Remove tag from asset
[**assets_tags_id_post**](AssetsApi.md#assets_tags_id_post) | **POST** /assets/tags/{id} | Add tag to asset
[**assets_terms_id_delete**](AssetsApi.md#assets_terms_id_delete) | **DELETE** /assets/terms/{id} | Remove glossary term from asset
[**assets_terms_id_get**](AssetsApi.md#assets_terms_id_get) | **GET** /assets/terms/{id} | Get asset&#39;s glossary terms
[**assets_terms_id_post**](AssetsApi.md#assets_terms_id_post) | **POST** /assets/terms/{id} | Add glossary terms to asset


# **assets_by_glossary_term_term_id_get**
> Dict[str, object] assets_by_glossary_term_term_id_get(term_id, limit=limit, offset=offset)

**Synchronous variant:** `assets_by_glossary_term_term_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get assets by glossary term

Retrieve all assets associated with a specific glossary term

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
    api_instance = marmot.generated.AssetsApi(api_client)
    term_id = 'term_id_example' # str | Glossary Term ID
    limit = 20 # int | Maximum number of assets (optional) (default to 20)
    offset = 0 # int | Pagination offset (optional) (default to 0)

    try:
        # Get assets by glossary term
        api_response = await api_instance.assets_by_glossary_term_term_id_get(term_id, limit=limit, offset=offset)
        print("The response of AssetsApi->assets_by_glossary_term_term_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_by_glossary_term_term_id_get: %s\n" % e)
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

# **assets_documentation_batch_post**
> BatchDocumentationResponse assets_documentation_batch_post(batch_documentation_request)

**Synchronous variant:** `assets_documentation_batch_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)
    batch_documentation_request = marmot.generated.BatchDocumentationRequest() # BatchDocumentationRequest | Batch documentation request

    try:
        # Batch create documentation
        api_response = await api_instance.assets_documentation_batch_post(batch_documentation_request)
        print("The response of AssetsApi->assets_documentation_batch_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_documentation_batch_post: %s\n" % e)
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

# **assets_documentation_mrn_get**
> List[Documentation] assets_documentation_mrn_get(mrn)

**Synchronous variant:** `assets_documentation_mrn_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)
    mrn = 'mrn_example' # str | Asset MRN

    try:
        # Get asset documentation
        api_response = await api_instance.assets_documentation_mrn_get(mrn)
        print("The response of AssetsApi->assets_documentation_mrn_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_documentation_mrn_get: %s\n" % e)
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

# **assets_documentation_post**
> Documentation assets_documentation_post(documentation_create_request)

**Synchronous variant:** `assets_documentation_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)
    documentation_create_request = marmot.generated.DocumentationCreateRequest() # DocumentationCreateRequest | Documentation creation request

    try:
        # Create asset documentation
        api_response = await api_instance.assets_documentation_post(documentation_create_request)
        print("The response of AssetsApi->assets_documentation_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_documentation_post: %s\n" % e)
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

# **assets_id_delete**
> assets_id_delete(id)

**Synchronous variant:** `assets_id_delete_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete an asset

Delete an asset from the system

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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID

    try:
        # Delete an asset
        await api_instance.assets_id_delete(id)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_id_delete: %s\n" % e)
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

# **assets_id_get**
> Asset assets_id_get(id)

**Synchronous variant:** `assets_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID

    try:
        # Get an asset by ID
        api_response = await api_instance.assets_id_get(id)
        print("The response of AssetsApi->assets_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_id_get: %s\n" % e)
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

# **assets_id_put**
> Asset assets_id_put(id, update_asset_request)

**Synchronous variant:** `assets_id_put_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID
    update_asset_request = marmot.generated.UpdateAssetRequest() # UpdateAssetRequest | Asset update request

    try:
        # Update an asset
        api_response = await api_instance.assets_id_put(id, update_asset_request)
        print("The response of AssetsApi->assets_id_put:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_id_put: %s\n" % e)
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

# **assets_id_run_history_get**
> RunHistoryResponse assets_id_run_history_get(id, limit=limit, offset=offset)

**Synchronous variant:** `assets_id_run_history_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID
    limit = 10 # int | Number of items per page (optional) (default to 10)
    offset = 0 # int | Number of items to skip (optional) (default to 0)

    try:
        # Get asset run history
        api_response = await api_instance.assets_id_run_history_get(id, limit=limit, offset=offset)
        print("The response of AssetsApi->assets_id_run_history_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_id_run_history_get: %s\n" % e)
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

# **assets_id_run_history_histogram_get**
> HistogramResponse assets_id_run_history_histogram_get(id, period=period)

**Synchronous variant:** `assets_id_run_history_histogram_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID
    period = '30d' # str | Time period (7d, 30d, 90d) (optional) (default to '30d')

    try:
        # Get asset run history histogram
        api_response = await api_instance.assets_id_run_history_histogram_get(id, period=period)
        print("The response of AssetsApi->assets_id_run_history_histogram_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_id_run_history_histogram_get: %s\n" % e)
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

# **assets_lookup_type_service_name_get**
> Asset assets_lookup_type_service_name_get(type, service, name)

**Synchronous variant:** `assets_lookup_type_service_name_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)
    type = 'type_example' # str | Asset type
    service = 'service_example' # str | Service/Provider name
    name = 'name_example' # str | Asset name

    try:
        # Lookup asset by type, service, and name
        api_response = await api_instance.assets_lookup_type_service_name_get(type, service, name)
        print("The response of AssetsApi->assets_lookup_type_service_name_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_lookup_type_service_name_get: %s\n" % e)
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

# **assets_match_pattern_get**
> List[Asset] assets_match_pattern_get(pattern, type)

**Synchronous variant:** `assets_match_pattern_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)
    pattern = 'pattern_example' # str | Asset pattern to match
    type = 'type_example' # str | Asset type

    try:
        # Match asset pattern
        api_response = await api_instance.assets_match_pattern_get(pattern, type)
        print("The response of AssetsApi->assets_match_pattern_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_match_pattern_get: %s\n" % e)
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

# **assets_my_assets_get**
> AssetSearchResponse assets_my_assets_get(limit=limit, offset=offset)

**Synchronous variant:** `assets_my_assets_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)
    limit = 20 # int | Limit (optional) (default to 20)
    offset = 0 # int | Offset (optional) (default to 0)

    try:
        # Get user's assets
        api_response = await api_instance.assets_my_assets_get(limit=limit, offset=offset)
        print("The response of AssetsApi->assets_my_assets_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_my_assets_get: %s\n" % e)
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

# **assets_post**
> Asset assets_post(create_asset_request)

**Synchronous variant:** `assets_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)
    create_asset_request = marmot.generated.CreateAssetRequest() # CreateAssetRequest | Asset creation request

    try:
        # Create a new asset
        api_response = await api_instance.assets_post(create_asset_request)
        print("The response of AssetsApi->assets_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_post: %s\n" % e)
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

# **assets_preview_id_get**
> PreviewResponse assets_preview_id_get(id)

**Synchronous variant:** `assets_preview_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID

    try:
        # Get preview data for an asset
        api_response = await api_instance.assets_preview_id_get(id)
        print("The response of AssetsApi->assets_preview_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_preview_id_get: %s\n" % e)
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

# **assets_qualified_name_qualified_name_get**
> Asset assets_qualified_name_qualified_name_get(qualified_name)

**Synchronous variant:** `assets_qualified_name_qualified_name_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)
    qualified_name = 'qualified_name_example' # str | Asset qualified name

    try:
        # Get an asset by qualified name
        api_response = await api_instance.assets_qualified_name_qualified_name_get(qualified_name)
        print("The response of AssetsApi->assets_qualified_name_qualified_name_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_qualified_name_qualified_name_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **qualified_name** | **str**| Asset qualified name | 

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

# **assets_search_get**
> AssetSearchResponse assets_search_get(q=q, types=types, services=services, tags=tags, limit=limit, offset=offset, calculate_counts=calculate_counts)

**Synchronous variant:** `assets_search_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
        api_response = await api_instance.assets_search_get(q=q, types=types, services=services, tags=tags, limit=limit, offset=offset, calculate_counts=calculate_counts)
        print("The response of AssetsApi->assets_search_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_search_get: %s\n" % e)
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

# **assets_suggestions_metadata_fields_get**
> List[MetadataFieldSuggestion] assets_suggestions_metadata_fields_get()

**Synchronous variant:** `assets_suggestions_metadata_fields_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)

    try:
        # Get metadata field suggestions
        api_response = await api_instance.assets_suggestions_metadata_fields_get()
        print("The response of AssetsApi->assets_suggestions_metadata_fields_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_suggestions_metadata_fields_get: %s\n" % e)
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

# **assets_suggestions_metadata_values_get**
> List[MetadataValueSuggestion] assets_suggestions_metadata_values_get(var_field, prefix=prefix, limit=limit)

**Synchronous variant:** `assets_suggestions_metadata_values_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)
    var_field = 'var_field_example' # str | Metadata field name
    prefix = 'prefix_example' # str | Value prefix to filter by (optional)
    limit = 10 # int | Maximum number of suggestions (optional) (default to 10)

    try:
        # Get metadata value suggestions
        api_response = await api_instance.assets_suggestions_metadata_values_get(var_field, prefix=prefix, limit=limit)
        print("The response of AssetsApi->assets_suggestions_metadata_values_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_suggestions_metadata_values_get: %s\n" % e)
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

# **assets_suggestions_tags_get**
> List[str] assets_suggestions_tags_get(prefix=prefix, limit=limit)

**Synchronous variant:** `assets_suggestions_tags_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get tag suggestions

Get suggestions for asset tags

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
    api_instance = marmot.generated.AssetsApi(api_client)
    prefix = 'prefix_example' # str | Tag prefix to filter by (optional)
    limit = 10 # int | Maximum number of suggestions (optional) (default to 10)

    try:
        # Get tag suggestions
        api_response = await api_instance.assets_suggestions_tags_get(prefix=prefix, limit=limit)
        print("The response of AssetsApi->assets_suggestions_tags_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_suggestions_tags_get: %s\n" % e)
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

# **assets_summary_get**
> AssetSummaryResponse assets_summary_get()

**Synchronous variant:** `assets_summary_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)

    try:
        # Get asset summary
        api_response = await api_instance.assets_summary_get()
        print("The response of AssetsApi->assets_summary_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_summary_get: %s\n" % e)
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

# **assets_tags_id_delete**
> Asset assets_tags_id_delete(id, tag_request)

**Synchronous variant:** `assets_tags_id_delete_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID
    tag_request = marmot.generated.TagRequest() # TagRequest | Tag to remove

    try:
        # Remove tag from asset
        api_response = await api_instance.assets_tags_id_delete(id, tag_request)
        print("The response of AssetsApi->assets_tags_id_delete:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_tags_id_delete: %s\n" % e)
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

# **assets_tags_id_post**
> Asset assets_tags_id_post(id, tag_request)

**Synchronous variant:** `assets_tags_id_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID
    tag_request = marmot.generated.TagRequest() # TagRequest | Tag to add

    try:
        # Add tag to asset
        api_response = await api_instance.assets_tags_id_post(id, tag_request)
        print("The response of AssetsApi->assets_tags_id_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_tags_id_post: %s\n" % e)
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

# **assets_terms_id_delete**
> List[AssetTerm] assets_terms_id_delete(id, remove_term_request)

**Synchronous variant:** `assets_terms_id_delete_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID
    remove_term_request = marmot.generated.RemoveTermRequest() # RemoveTermRequest | Term ID to remove

    try:
        # Remove glossary term from asset
        api_response = await api_instance.assets_terms_id_delete(id, remove_term_request)
        print("The response of AssetsApi->assets_terms_id_delete:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_terms_id_delete: %s\n" % e)
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

# **assets_terms_id_get**
> List[AssetTerm] assets_terms_id_get(id)

**Synchronous variant:** `assets_terms_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID

    try:
        # Get asset's glossary terms
        api_response = await api_instance.assets_terms_id_get(id)
        print("The response of AssetsApi->assets_terms_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_terms_id_get: %s\n" % e)
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

# **assets_terms_id_post**
> List[AssetTerm] assets_terms_id_post(id, add_terms_request)

**Synchronous variant:** `assets_terms_id_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.AssetsApi(api_client)
    id = 'id_example' # str | Asset ID
    add_terms_request = marmot.generated.AddTermsRequest() # AddTermsRequest | Term IDs to add

    try:
        # Add glossary terms to asset
        api_response = await api_instance.assets_terms_id_post(id, add_terms_request)
        print("The response of AssetsApi->assets_terms_id_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetsApi->assets_terms_id_post: %s\n" % e)
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

