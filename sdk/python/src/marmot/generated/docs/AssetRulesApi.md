# marmot.generated.AssetRulesApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**delete_asset_rules_id**](AssetRulesApi.md#delete_asset_rules_id) | **DELETE** /api/v1/asset-rules/{id} | Delete an asset rule
[**get_asset_rules_assets_id**](AssetRulesApi.md#get_asset_rules_assets_id) | **GET** /api/v1/asset-rules/assets/{id} | Get assets matched by a rule
[**get_asset_rules_id**](AssetRulesApi.md#get_asset_rules_id) | **GET** /api/v1/asset-rules/{id} | Get an asset rule
[**get_asset_rules_list**](AssetRulesApi.md#get_asset_rules_list) | **GET** /api/v1/asset-rules/list | List asset rules
[**get_asset_rules_search**](AssetRulesApi.md#get_asset_rules_search) | **GET** /api/v1/asset-rules/search | Search asset rules
[**post_asset_rules**](AssetRulesApi.md#post_asset_rules) | **POST** /api/v1/asset-rules/ | Create an asset rule
[**post_asset_rules_preview**](AssetRulesApi.md#post_asset_rules_preview) | **POST** /api/v1/asset-rules/preview | Preview an asset rule
[**put_asset_rules_id**](AssetRulesApi.md#put_asset_rules_id) | **PUT** /api/v1/asset-rules/{id} | Update an asset rule


# **delete_asset_rules_id**
> delete_asset_rules_id(id)

**Synchronous variant:** `delete_asset_rules_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete an asset rule

Delete an asset rule by ID

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
    api_instance = marmot.generated.AssetRulesApi(api_client)
    id = 'id_example' # str | Asset rule ID

    try:
        # Delete an asset rule
        await api_instance.delete_asset_rules_id(id)
    except Exception as e:
        print("Exception when calling AssetRulesApi->delete_asset_rules_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Asset rule ID | 

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
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **get_asset_rules_assets_id**
> Dict[str, object] get_asset_rules_assets_id(id, limit=limit, offset=offset)

**Synchronous variant:** `get_asset_rules_assets_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get assets matched by a rule

Get the list of asset IDs matched by an asset rule

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
    api_instance = marmot.generated.AssetRulesApi(api_client)
    id = 'id_example' # str | Asset rule ID
    limit = 50 # int | Number of items to return (optional) (default to 50)
    offset = 0 # int | Number of items to skip (optional) (default to 0)

    try:
        # Get assets matched by a rule
        api_response = await api_instance.get_asset_rules_assets_id(id, limit=limit, offset=offset)
        print("The response of AssetRulesApi->get_asset_rules_assets_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetRulesApi->get_asset_rules_assets_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Asset rule ID | 
 **limit** | **int**| Number of items to return | [optional] [default to 50]
 **offset** | **int**| Number of items to skip | [optional] [default to 0]

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
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **get_asset_rules_id**
> AssetRule get_asset_rules_id(id)

**Synchronous variant:** `get_asset_rules_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get an asset rule

Get an asset rule by ID

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.asset_rule import AssetRule
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
    api_instance = marmot.generated.AssetRulesApi(api_client)
    id = 'id_example' # str | Asset rule ID

    try:
        # Get an asset rule
        api_response = await api_instance.get_asset_rules_id(id)
        print("The response of AssetRulesApi->get_asset_rules_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetRulesApi->get_asset_rules_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Asset rule ID | 

### Return type

[**AssetRule**](AssetRule.md)

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

# **get_asset_rules_list**
> AssetRuleListResult get_asset_rules_list(limit=limit, offset=offset)

**Synchronous variant:** `get_asset_rules_list_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List asset rules

List all asset rules with pagination

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.asset_rule_list_result import AssetRuleListResult
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
    api_instance = marmot.generated.AssetRulesApi(api_client)
    limit = 50 # int | Number of items to return (optional) (default to 50)
    offset = 0 # int | Number of items to skip (optional) (default to 0)

    try:
        # List asset rules
        api_response = await api_instance.get_asset_rules_list(limit=limit, offset=offset)
        print("The response of AssetRulesApi->get_asset_rules_list:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetRulesApi->get_asset_rules_list: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **limit** | **int**| Number of items to return | [optional] [default to 50]
 **offset** | **int**| Number of items to skip | [optional] [default to 0]

### Return type

[**AssetRuleListResult**](AssetRuleListResult.md)

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

# **get_asset_rules_search**
> AssetRuleListResult get_asset_rules_search(query=query, limit=limit, offset=offset)

**Synchronous variant:** `get_asset_rules_search_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Search asset rules

Search asset rules by name

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.asset_rule_list_result import AssetRuleListResult
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
    api_instance = marmot.generated.AssetRulesApi(api_client)
    query = 'query_example' # str | Search query (optional)
    limit = 50 # int | Number of items to return (optional) (default to 50)
    offset = 0 # int | Number of items to skip (optional) (default to 0)

    try:
        # Search asset rules
        api_response = await api_instance.get_asset_rules_search(query=query, limit=limit, offset=offset)
        print("The response of AssetRulesApi->get_asset_rules_search:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetRulesApi->get_asset_rules_search: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **query** | **str**| Search query | [optional] 
 **limit** | **int**| Number of items to return | [optional] [default to 50]
 **offset** | **int**| Number of items to skip | [optional] [default to 0]

### Return type

[**AssetRuleListResult**](AssetRuleListResult.md)

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

# **post_asset_rules**
> AssetRule post_asset_rules(create_asset_rule_request)

**Synchronous variant:** `post_asset_rules_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Create an asset rule

Create a new asset rule that applies enrichments to matching assets

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.asset_rule import AssetRule
from marmot.generated.models.create_asset_rule_request import CreateAssetRuleRequest
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
    api_instance = marmot.generated.AssetRulesApi(api_client)
    create_asset_rule_request = marmot.generated.CreateAssetRuleRequest() # CreateAssetRuleRequest | Asset rule creation request

    try:
        # Create an asset rule
        api_response = await api_instance.post_asset_rules(create_asset_rule_request)
        print("The response of AssetRulesApi->post_asset_rules:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetRulesApi->post_asset_rules: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **create_asset_rule_request** | [**CreateAssetRuleRequest**](CreateAssetRuleRequest.md)| Asset rule creation request | 

### Return type

[**AssetRule**](AssetRule.md)

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
**409** | Conflict |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **post_asset_rules_preview**
> RulePreview post_asset_rules_preview(preview_request)

**Synchronous variant:** `post_asset_rules_preview_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Preview an asset rule

Preview which assets would match a rule configuration

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.preview_request import PreviewRequest
from marmot.generated.models.rule_preview import RulePreview
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
    api_instance = marmot.generated.AssetRulesApi(api_client)
    preview_request = marmot.generated.PreviewRequest() # PreviewRequest | Rule preview request

    try:
        # Preview an asset rule
        api_response = await api_instance.post_asset_rules_preview(preview_request)
        print("The response of AssetRulesApi->post_asset_rules_preview:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetRulesApi->post_asset_rules_preview: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **preview_request** | [**PreviewRequest**](PreviewRequest.md)| Rule preview request | 

### Return type

[**RulePreview**](RulePreview.md)

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

# **put_asset_rules_id**
> AssetRule put_asset_rules_id(id, update_asset_rule_request)

**Synchronous variant:** `put_asset_rules_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Update an asset rule

Update an existing asset rule

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.asset_rule import AssetRule
from marmot.generated.models.update_asset_rule_request import UpdateAssetRuleRequest
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
    api_instance = marmot.generated.AssetRulesApi(api_client)
    id = 'id_example' # str | Asset rule ID
    update_asset_rule_request = marmot.generated.UpdateAssetRuleRequest() # UpdateAssetRuleRequest | Asset rule update request

    try:
        # Update an asset rule
        api_response = await api_instance.put_asset_rules_id(id, update_asset_rule_request)
        print("The response of AssetRulesApi->put_asset_rules_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AssetRulesApi->put_asset_rules_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Asset rule ID | 
 **update_asset_rule_request** | [**UpdateAssetRuleRequest**](UpdateAssetRuleRequest.md)| Asset rule update request | 

### Return type

[**AssetRule**](AssetRule.md)

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
**409** | Conflict |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

