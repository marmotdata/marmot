# marmot.generated.ProductsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**delete_products_assets_id_asset_id**](ProductsApi.md#delete_products_assets_id_asset_id) | **DELETE** /api/v1/products/assets/{id}/{assetId} | Remove data product asset
[**delete_products_id**](ProductsApi.md#delete_products_id) | **DELETE** /api/v1/products/{id} | Delete data product
[**delete_products_images_id_purpose**](ProductsApi.md#delete_products_images_id_purpose) | **DELETE** /api/v1/products/images/{id}/{purpose} | Delete product image
[**delete_products_rules_id_rule_id**](ProductsApi.md#delete_products_rules_id_rule_id) | **DELETE** /api/v1/products/rules/{id}/{ruleId} | Delete data product rule
[**get_products_assets_id**](ProductsApi.md#get_products_assets_id) | **GET** /api/v1/products/assets/{id} | Get data product assets
[**get_products_id**](ProductsApi.md#get_products_id) | **GET** /api/v1/products/{id} | Get data product
[**get_products_images_id**](ProductsApi.md#get_products_images_id) | **GET** /api/v1/products/images/{id} | List product images
[**get_products_images_id_purpose**](ProductsApi.md#get_products_images_id_purpose) | **GET** /api/v1/products/images/{id}/{purpose} | Get product image
[**get_products_list**](ProductsApi.md#get_products_list) | **GET** /api/v1/products/list | List data products
[**get_products_resolved_assets_id**](ProductsApi.md#get_products_resolved_assets_id) | **GET** /api/v1/products/resolved-assets/{id} | Get resolved data product assets
[**get_products_rules_id**](ProductsApi.md#get_products_rules_id) | **GET** /api/v1/products/rules/{id} | Get data product rules
[**get_products_search**](ProductsApi.md#get_products_search) | **GET** /api/v1/products/search | Search data products
[**post_products**](ProductsApi.md#post_products) | **POST** /api/v1/products/ | Create data product
[**post_products_assets_id**](ProductsApi.md#post_products_assets_id) | **POST** /api/v1/products/assets/{id} | Add data product assets
[**post_products_images_id_purpose**](ProductsApi.md#post_products_images_id_purpose) | **POST** /api/v1/products/images/{id}/{purpose} | Upload product image
[**post_products_rule_preview**](ProductsApi.md#post_products_rule_preview) | **POST** /api/v1/products/rule-preview | Preview data product rule
[**post_products_rules_id**](ProductsApi.md#post_products_rules_id) | **POST** /api/v1/products/rules/{id} | Create data product rule
[**put_products_id**](ProductsApi.md#put_products_id) | **PUT** /api/v1/products/{id} | Update data product
[**put_products_rules_id_rule_id**](ProductsApi.md#put_products_rules_id_rule_id) | **PUT** /api/v1/products/rules/{id}/{ruleId} | Update data product rule


# **delete_products_assets_id_asset_id**
> Dict[str, str] delete_products_assets_id_asset_id(id, asset_id)

**Synchronous variant:** `delete_products_assets_id_asset_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Remove data product asset

Remove a manually added asset from a data product

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    asset_id = 'asset_id_example' # str | Asset ID

    try:
        # Remove data product asset
        api_response = await api_instance.delete_products_assets_id_asset_id(id, asset_id)
        print("The response of ProductsApi->delete_products_assets_id_asset_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->delete_products_assets_id_asset_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Data Product ID | 
 **asset_id** | **str**| Asset ID | 

### Return type

**Dict[str, str]**

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

# **delete_products_id**
> Dict[str, str] delete_products_id(id)

**Synchronous variant:** `delete_products_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete data product

Delete a data product by ID

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID

    try:
        # Delete data product
        api_response = await api_instance.delete_products_id(id)
        print("The response of ProductsApi->delete_products_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->delete_products_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Data Product ID | 

### Return type

**Dict[str, str]**

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

# **delete_products_images_id_purpose**
> Dict[str, str] delete_products_images_id_purpose(id, purpose)

**Synchronous variant:** `delete_products_images_id_purpose_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete product image

Delete an icon or header image for a data product

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    purpose = 'purpose_example' # str | Image purpose (icon or header)

    try:
        # Delete product image
        api_response = await api_instance.delete_products_images_id_purpose(id, purpose)
        print("The response of ProductsApi->delete_products_images_id_purpose:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->delete_products_images_id_purpose: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Data Product ID | 
 **purpose** | **str**| Image purpose (icon or header) | 

### Return type

**Dict[str, str]**

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

# **delete_products_rules_id_rule_id**
> Dict[str, str] delete_products_rules_id_rule_id(id, rule_id)

**Synchronous variant:** `delete_products_rules_id_rule_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete data product rule

Delete a membership rule from a data product

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    rule_id = 'rule_id_example' # str | Rule ID

    try:
        # Delete data product rule
        api_response = await api_instance.delete_products_rules_id_rule_id(id, rule_id)
        print("The response of ProductsApi->delete_products_rules_id_rule_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->delete_products_rules_id_rule_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Data Product ID | 
 **rule_id** | **str**| Rule ID | 

### Return type

**Dict[str, str]**

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

# **get_products_assets_id**
> DataProductAssetsResult get_products_assets_id(id, limit=limit, offset=offset)

**Synchronous variant:** `get_products_assets_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get data product assets

Get the manually added assets of a data product

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.data_product_assets_result import DataProductAssetsResult
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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    limit = 20 # int | Maximum number of assets to return (optional) (default to 20)
    offset = 0 # int | Number of assets to skip (optional) (default to 0)

    try:
        # Get data product assets
        api_response = await api_instance.get_products_assets_id(id, limit=limit, offset=offset)
        print("The response of ProductsApi->get_products_assets_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->get_products_assets_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Data Product ID | 
 **limit** | **int**| Maximum number of assets to return | [optional] [default to 20]
 **offset** | **int**| Number of assets to skip | [optional] [default to 0]

### Return type

[**DataProductAssetsResult**](DataProductAssetsResult.md)

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

# **get_products_id**
> DataProduct get_products_id(id)

**Synchronous variant:** `get_products_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get data product

Get a data product by ID

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.data_product import DataProduct
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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID

    try:
        # Get data product
        api_response = await api_instance.get_products_id(id)
        print("The response of ProductsApi->get_products_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->get_products_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Data Product ID | 

### Return type

[**DataProduct**](DataProduct.md)

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

# **get_products_images_id**
> Dict[str, object] get_products_images_id(id)

**Synchronous variant:** `get_products_images_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List product images

List all images for a data product

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID

    try:
        # List product images
        api_response = await api_instance.get_products_images_id(id)
        print("The response of ProductsApi->get_products_images_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->get_products_images_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Data Product ID | 

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

# **get_products_images_id_purpose**
> bytes get_products_images_id_purpose(id, purpose)

**Synchronous variant:** `get_products_images_id_purpose_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get product image

Get an icon or header image for a data product

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    purpose = 'purpose_example' # str | Image purpose (icon or header)

    try:
        # Get product image
        api_response = await api_instance.get_products_images_id_purpose(id, purpose)
        print("The response of ProductsApi->get_products_images_id_purpose:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->get_products_images_id_purpose: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Data Product ID | 
 **purpose** | **str**| Image purpose (icon or header) | 

### Return type

**bytes**

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: image/jpeg, image/png, image/gif, image/webp

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **get_products_list**
> DataProductListResult get_products_list(limit=limit, offset=offset)

**Synchronous variant:** `get_products_list_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List data products

Retrieve a paginated list of data products

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.data_product_list_result import DataProductListResult
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
    api_instance = marmot.generated.ProductsApi(api_client)
    limit = 20 # int | Maximum number of data products to return (optional) (default to 20)
    offset = 0 # int | Number of data products to skip (optional) (default to 0)

    try:
        # List data products
        api_response = await api_instance.get_products_list(limit=limit, offset=offset)
        print("The response of ProductsApi->get_products_list:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->get_products_list: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **limit** | **int**| Maximum number of data products to return | [optional] [default to 20]
 **offset** | **int**| Number of data products to skip | [optional] [default to 0]

### Return type

[**DataProductListResult**](DataProductListResult.md)

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

# **get_products_resolved_assets_id**
> DataProductResolvedAssets get_products_resolved_assets_id(id, limit=limit, offset=offset)

**Synchronous variant:** `get_products_resolved_assets_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get resolved data product assets

Get all assets of a data product, both manually added and matched by rules

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.data_product_resolved_assets import DataProductResolvedAssets
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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    limit = 20 # int | Maximum number of assets to return (optional) (default to 20)
    offset = 0 # int | Number of assets to skip (optional) (default to 0)

    try:
        # Get resolved data product assets
        api_response = await api_instance.get_products_resolved_assets_id(id, limit=limit, offset=offset)
        print("The response of ProductsApi->get_products_resolved_assets_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->get_products_resolved_assets_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Data Product ID | 
 **limit** | **int**| Maximum number of assets to return | [optional] [default to 20]
 **offset** | **int**| Number of assets to skip | [optional] [default to 0]

### Return type

[**DataProductResolvedAssets**](DataProductResolvedAssets.md)

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

# **get_products_rules_id**
> DataProductRulesResponse get_products_rules_id(id)

**Synchronous variant:** `get_products_rules_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get data product rules

Get the membership rules of a data product

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.data_product_rules_response import DataProductRulesResponse
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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID

    try:
        # Get data product rules
        api_response = await api_instance.get_products_rules_id(id)
        print("The response of ProductsApi->get_products_rules_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->get_products_rules_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Data Product ID | 

### Return type

[**DataProductRulesResponse**](DataProductRulesResponse.md)

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

# **get_products_search**
> DataProductListResult get_products_search(q=q, tags=tags, limit=limit, offset=offset)

**Synchronous variant:** `get_products_search_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Search data products

Search data products by name, description, and tags

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.data_product_list_result import DataProductListResult
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
    api_instance = marmot.generated.ProductsApi(api_client)
    q = 'q_example' # str | Search query (optional)
    tags = 'tags_example' # str | Comma-separated list of tags to filter by (optional)
    limit = 20 # int | Maximum number of data products to return (optional) (default to 20)
    offset = 0 # int | Number of data products to skip (optional) (default to 0)

    try:
        # Search data products
        api_response = await api_instance.get_products_search(q=q, tags=tags, limit=limit, offset=offset)
        print("The response of ProductsApi->get_products_search:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->get_products_search: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **q** | **str**| Search query | [optional] 
 **tags** | **str**| Comma-separated list of tags to filter by | [optional] 
 **limit** | **int**| Maximum number of data products to return | [optional] [default to 20]
 **offset** | **int**| Number of data products to skip | [optional] [default to 0]

### Return type

[**DataProductListResult**](DataProductListResult.md)

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

# **post_products**
> DataProduct post_products(create_data_product_request)

**Synchronous variant:** `post_products_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Create data product

Create a new data product with owners and optional membership rules

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.create_data_product_request import CreateDataProductRequest
from marmot.generated.models.data_product import DataProduct
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
    api_instance = marmot.generated.ProductsApi(api_client)
    create_data_product_request = marmot.generated.CreateDataProductRequest() # CreateDataProductRequest | Data product to create

    try:
        # Create data product
        api_response = await api_instance.post_products(create_data_product_request)
        print("The response of ProductsApi->post_products:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->post_products: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **create_data_product_request** | [**CreateDataProductRequest**](CreateDataProductRequest.md)| Data product to create | 

### Return type

[**DataProduct**](DataProduct.md)

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
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **post_products_assets_id**
> Dict[str, str] post_products_assets_id(id, add_data_product_assets_request)

**Synchronous variant:** `post_products_assets_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Add data product assets

Manually add assets to a data product

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.add_data_product_assets_request import AddDataProductAssetsRequest
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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    add_data_product_assets_request = marmot.generated.AddDataProductAssetsRequest() # AddDataProductAssetsRequest | Asset IDs to add

    try:
        # Add data product assets
        api_response = await api_instance.post_products_assets_id(id, add_data_product_assets_request)
        print("The response of ProductsApi->post_products_assets_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->post_products_assets_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Data Product ID | 
 **add_data_product_assets_request** | [**AddDataProductAssetsRequest**](AddDataProductAssetsRequest.md)| Asset IDs to add | 

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
**400** | Bad Request |  -  |
**401** | Unauthorized |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **post_products_images_id_purpose**
> ProductImageMeta post_products_images_id_purpose(id, purpose, file)

**Synchronous variant:** `post_products_images_id_purpose_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Upload product image

Upload an icon or header image for a data product

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.product_image_meta import ProductImageMeta
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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    purpose = 'purpose_example' # str | Image purpose (icon or header)
    file = None # bytes | Image file

    try:
        # Upload product image
        api_response = await api_instance.post_products_images_id_purpose(id, purpose, file)
        print("The response of ProductsApi->post_products_images_id_purpose:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->post_products_images_id_purpose: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Data Product ID | 
 **purpose** | **str**| Image purpose (icon or header) | 
 **file** | **bytes**| Image file | 

### Return type

[**ProductImageMeta**](ProductImageMeta.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: multipart/form-data
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**400** | Bad Request |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **post_products_rule_preview**
> DataProductRulePreview post_products_rule_preview(data_product_rule_request, limit=limit)

**Synchronous variant:** `post_products_rule_preview_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Preview data product rule

Preview which assets would match a rule configuration

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.data_product_rule_preview import DataProductRulePreview
from marmot.generated.models.data_product_rule_request import DataProductRuleRequest
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
    api_instance = marmot.generated.ProductsApi(api_client)
    data_product_rule_request = marmot.generated.DataProductRuleRequest() # DataProductRuleRequest | Rule to preview
    limit = 20 # int | Maximum number of matching assets to return (optional) (default to 20)

    try:
        # Preview data product rule
        api_response = await api_instance.post_products_rule_preview(data_product_rule_request, limit=limit)
        print("The response of ProductsApi->post_products_rule_preview:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->post_products_rule_preview: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **data_product_rule_request** | [**DataProductRuleRequest**](DataProductRuleRequest.md)| Rule to preview | 
 **limit** | **int**| Maximum number of matching assets to return | [optional] [default to 20]

### Return type

[**DataProductRulePreview**](DataProductRulePreview.md)

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

# **post_products_rules_id**
> DataProductRule post_products_rules_id(id, data_product_rule_request)

**Synchronous variant:** `post_products_rules_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Create data product rule

Create a membership rule for a data product

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.data_product_rule import DataProductRule
from marmot.generated.models.data_product_rule_request import DataProductRuleRequest
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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    data_product_rule_request = marmot.generated.DataProductRuleRequest() # DataProductRuleRequest | Rule to create

    try:
        # Create data product rule
        api_response = await api_instance.post_products_rules_id(id, data_product_rule_request)
        print("The response of ProductsApi->post_products_rules_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->post_products_rules_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Data Product ID | 
 **data_product_rule_request** | [**DataProductRuleRequest**](DataProductRuleRequest.md)| Rule to create | 

### Return type

[**DataProductRule**](DataProductRule.md)

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
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **put_products_id**
> DataProduct put_products_id(id, update_data_product_request)

**Synchronous variant:** `put_products_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Update data product

Update an existing data product

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.data_product import DataProduct
from marmot.generated.models.update_data_product_request import UpdateDataProductRequest
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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    update_data_product_request = marmot.generated.UpdateDataProductRequest() # UpdateDataProductRequest | Fields to update

    try:
        # Update data product
        api_response = await api_instance.put_products_id(id, update_data_product_request)
        print("The response of ProductsApi->put_products_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->put_products_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Data Product ID | 
 **update_data_product_request** | [**UpdateDataProductRequest**](UpdateDataProductRequest.md)| Fields to update | 

### Return type

[**DataProduct**](DataProduct.md)

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

# **put_products_rules_id_rule_id**
> DataProductRule put_products_rules_id_rule_id(id, rule_id, data_product_rule_request)

**Synchronous variant:** `put_products_rules_id_rule_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Update data product rule

Update a membership rule of a data product

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.data_product_rule import DataProductRule
from marmot.generated.models.data_product_rule_request import DataProductRuleRequest
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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    rule_id = 'rule_id_example' # str | Rule ID
    data_product_rule_request = marmot.generated.DataProductRuleRequest() # DataProductRuleRequest | Rule fields to update

    try:
        # Update data product rule
        api_response = await api_instance.put_products_rules_id_rule_id(id, rule_id, data_product_rule_request)
        print("The response of ProductsApi->put_products_rules_id_rule_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->put_products_rules_id_rule_id: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Data Product ID | 
 **rule_id** | **str**| Rule ID | 
 **data_product_rule_request** | [**DataProductRuleRequest**](DataProductRuleRequest.md)| Rule fields to update | 

### Return type

[**DataProductRule**](DataProductRule.md)

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

