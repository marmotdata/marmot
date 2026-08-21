# marmot.generated.ProductsApi

All URIs are relative to */api/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**products_assets_id_asset_id_delete**](ProductsApi.md#products_assets_id_asset_id_delete) | **DELETE** /products/assets/{id}/{assetId} | Remove data product asset
[**products_assets_id_get**](ProductsApi.md#products_assets_id_get) | **GET** /products/assets/{id} | Get data product assets
[**products_assets_id_post**](ProductsApi.md#products_assets_id_post) | **POST** /products/assets/{id} | Add data product assets
[**products_id_delete**](ProductsApi.md#products_id_delete) | **DELETE** /products/{id} | Delete data product
[**products_id_get**](ProductsApi.md#products_id_get) | **GET** /products/{id} | Get data product
[**products_id_put**](ProductsApi.md#products_id_put) | **PUT** /products/{id} | Update data product
[**products_images_id_get**](ProductsApi.md#products_images_id_get) | **GET** /products/images/{id} | List product images
[**products_images_id_purpose_delete**](ProductsApi.md#products_images_id_purpose_delete) | **DELETE** /products/images/{id}/{purpose} | Delete product image
[**products_images_id_purpose_get**](ProductsApi.md#products_images_id_purpose_get) | **GET** /products/images/{id}/{purpose} | Get product image
[**products_images_id_purpose_post**](ProductsApi.md#products_images_id_purpose_post) | **POST** /products/images/{id}/{purpose} | Upload product image
[**products_list_get**](ProductsApi.md#products_list_get) | **GET** /products/list | List data products
[**products_post**](ProductsApi.md#products_post) | **POST** /products/ | Create data product
[**products_resolved_assets_id_get**](ProductsApi.md#products_resolved_assets_id_get) | **GET** /products/resolved-assets/{id} | Get resolved data product assets
[**products_rule_preview_post**](ProductsApi.md#products_rule_preview_post) | **POST** /products/rule-preview | Preview data product rule
[**products_rules_id_get**](ProductsApi.md#products_rules_id_get) | **GET** /products/rules/{id} | Get data product rules
[**products_rules_id_post**](ProductsApi.md#products_rules_id_post) | **POST** /products/rules/{id} | Create data product rule
[**products_rules_id_rule_id_delete**](ProductsApi.md#products_rules_id_rule_id_delete) | **DELETE** /products/rules/{id}/{ruleId} | Delete data product rule
[**products_rules_id_rule_id_put**](ProductsApi.md#products_rules_id_rule_id_put) | **PUT** /products/rules/{id}/{ruleId} | Update data product rule
[**products_search_get**](ProductsApi.md#products_search_get) | **GET** /products/search | Search data products


# **products_assets_id_asset_id_delete**
> Dict[str, str] products_assets_id_asset_id_delete(id, asset_id)

**Synchronous variant:** `products_assets_id_asset_id_delete_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Remove data product asset

Remove a manually added asset from a data product

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    asset_id = 'asset_id_example' # str | Asset ID

    try:
        # Remove data product asset
        api_response = await api_instance.products_assets_id_asset_id_delete(id, asset_id)
        print("The response of ProductsApi->products_assets_id_asset_id_delete:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->products_assets_id_asset_id_delete: %s\n" % e)
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

# **products_assets_id_get**
> DataProductAssetsResult products_assets_id_get(id, limit=limit, offset=offset)

**Synchronous variant:** `products_assets_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    limit = 20 # int | Maximum number of assets to return (optional) (default to 20)
    offset = 0 # int | Number of assets to skip (optional) (default to 0)

    try:
        # Get data product assets
        api_response = await api_instance.products_assets_id_get(id, limit=limit, offset=offset)
        print("The response of ProductsApi->products_assets_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->products_assets_id_get: %s\n" % e)
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

# **products_assets_id_post**
> Dict[str, str] products_assets_id_post(id, add_data_product_assets_request)

**Synchronous variant:** `products_assets_id_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    add_data_product_assets_request = marmot.generated.AddDataProductAssetsRequest() # AddDataProductAssetsRequest | Asset IDs to add

    try:
        # Add data product assets
        api_response = await api_instance.products_assets_id_post(id, add_data_product_assets_request)
        print("The response of ProductsApi->products_assets_id_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->products_assets_id_post: %s\n" % e)
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

# **products_id_delete**
> Dict[str, str] products_id_delete(id)

**Synchronous variant:** `products_id_delete_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete data product

Delete a data product by ID

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID

    try:
        # Delete data product
        api_response = await api_instance.products_id_delete(id)
        print("The response of ProductsApi->products_id_delete:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->products_id_delete: %s\n" % e)
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

# **products_id_get**
> DataProduct products_id_get(id)

**Synchronous variant:** `products_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID

    try:
        # Get data product
        api_response = await api_instance.products_id_get(id)
        print("The response of ProductsApi->products_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->products_id_get: %s\n" % e)
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

# **products_id_put**
> DataProduct products_id_put(id, update_data_product_request)

**Synchronous variant:** `products_id_put_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    update_data_product_request = marmot.generated.UpdateDataProductRequest() # UpdateDataProductRequest | Fields to update

    try:
        # Update data product
        api_response = await api_instance.products_id_put(id, update_data_product_request)
        print("The response of ProductsApi->products_id_put:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->products_id_put: %s\n" % e)
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

# **products_images_id_get**
> Dict[str, object] products_images_id_get(id)

**Synchronous variant:** `products_images_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List product images

List all images for a data product

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID

    try:
        # List product images
        api_response = await api_instance.products_images_id_get(id)
        print("The response of ProductsApi->products_images_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->products_images_id_get: %s\n" % e)
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

# **products_images_id_purpose_delete**
> Dict[str, str] products_images_id_purpose_delete(id, purpose)

**Synchronous variant:** `products_images_id_purpose_delete_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete product image

Delete an icon or header image for a data product

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    purpose = 'purpose_example' # str | Image purpose (icon or header)

    try:
        # Delete product image
        api_response = await api_instance.products_images_id_purpose_delete(id, purpose)
        print("The response of ProductsApi->products_images_id_purpose_delete:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->products_images_id_purpose_delete: %s\n" % e)
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

# **products_images_id_purpose_get**
> bytes products_images_id_purpose_get(id, purpose)

**Synchronous variant:** `products_images_id_purpose_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get product image

Get an icon or header image for a data product

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    purpose = 'purpose_example' # str | Image purpose (icon or header)

    try:
        # Get product image
        api_response = await api_instance.products_images_id_purpose_get(id, purpose)
        print("The response of ProductsApi->products_images_id_purpose_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->products_images_id_purpose_get: %s\n" % e)
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

# **products_images_id_purpose_post**
> ProductImageMeta products_images_id_purpose_post(id, purpose, file)

**Synchronous variant:** `products_images_id_purpose_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    purpose = 'purpose_example' # str | Image purpose (icon or header)
    file = None # bytes | Image file

    try:
        # Upload product image
        api_response = await api_instance.products_images_id_purpose_post(id, purpose, file)
        print("The response of ProductsApi->products_images_id_purpose_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->products_images_id_purpose_post: %s\n" % e)
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

# **products_list_get**
> DataProductListResult products_list_get(limit=limit, offset=offset)

**Synchronous variant:** `products_list_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.ProductsApi(api_client)
    limit = 20 # int | Maximum number of data products to return (optional) (default to 20)
    offset = 0 # int | Number of data products to skip (optional) (default to 0)

    try:
        # List data products
        api_response = await api_instance.products_list_get(limit=limit, offset=offset)
        print("The response of ProductsApi->products_list_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->products_list_get: %s\n" % e)
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

# **products_post**
> DataProduct products_post(create_data_product_request)

**Synchronous variant:** `products_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.ProductsApi(api_client)
    create_data_product_request = marmot.generated.CreateDataProductRequest() # CreateDataProductRequest | Data product to create

    try:
        # Create data product
        api_response = await api_instance.products_post(create_data_product_request)
        print("The response of ProductsApi->products_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->products_post: %s\n" % e)
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

# **products_resolved_assets_id_get**
> DataProductResolvedAssets products_resolved_assets_id_get(id, limit=limit, offset=offset)

**Synchronous variant:** `products_resolved_assets_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    limit = 20 # int | Maximum number of assets to return (optional) (default to 20)
    offset = 0 # int | Number of assets to skip (optional) (default to 0)

    try:
        # Get resolved data product assets
        api_response = await api_instance.products_resolved_assets_id_get(id, limit=limit, offset=offset)
        print("The response of ProductsApi->products_resolved_assets_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->products_resolved_assets_id_get: %s\n" % e)
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

# **products_rule_preview_post**
> DataProductRulePreview products_rule_preview_post(data_product_rule_request, limit=limit)

**Synchronous variant:** `products_rule_preview_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.ProductsApi(api_client)
    data_product_rule_request = marmot.generated.DataProductRuleRequest() # DataProductRuleRequest | Rule to preview
    limit = 20 # int | Maximum number of matching assets to return (optional) (default to 20)

    try:
        # Preview data product rule
        api_response = await api_instance.products_rule_preview_post(data_product_rule_request, limit=limit)
        print("The response of ProductsApi->products_rule_preview_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->products_rule_preview_post: %s\n" % e)
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

# **products_rules_id_get**
> DataProductRulesResponse products_rules_id_get(id)

**Synchronous variant:** `products_rules_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID

    try:
        # Get data product rules
        api_response = await api_instance.products_rules_id_get(id)
        print("The response of ProductsApi->products_rules_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->products_rules_id_get: %s\n" % e)
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

# **products_rules_id_post**
> DataProductRule products_rules_id_post(id, data_product_rule_request)

**Synchronous variant:** `products_rules_id_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    data_product_rule_request = marmot.generated.DataProductRuleRequest() # DataProductRuleRequest | Rule to create

    try:
        # Create data product rule
        api_response = await api_instance.products_rules_id_post(id, data_product_rule_request)
        print("The response of ProductsApi->products_rules_id_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->products_rules_id_post: %s\n" % e)
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

# **products_rules_id_rule_id_delete**
> Dict[str, str] products_rules_id_rule_id_delete(id, rule_id)

**Synchronous variant:** `products_rules_id_rule_id_delete_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete data product rule

Delete a membership rule from a data product

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    rule_id = 'rule_id_example' # str | Rule ID

    try:
        # Delete data product rule
        api_response = await api_instance.products_rules_id_rule_id_delete(id, rule_id)
        print("The response of ProductsApi->products_rules_id_rule_id_delete:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->products_rules_id_rule_id_delete: %s\n" % e)
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

# **products_rules_id_rule_id_put**
> DataProductRule products_rules_id_rule_id_put(id, rule_id, data_product_rule_request)

**Synchronous variant:** `products_rules_id_rule_id_put_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.ProductsApi(api_client)
    id = 'id_example' # str | Data Product ID
    rule_id = 'rule_id_example' # str | Rule ID
    data_product_rule_request = marmot.generated.DataProductRuleRequest() # DataProductRuleRequest | Rule fields to update

    try:
        # Update data product rule
        api_response = await api_instance.products_rules_id_rule_id_put(id, rule_id, data_product_rule_request)
        print("The response of ProductsApi->products_rules_id_rule_id_put:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->products_rules_id_rule_id_put: %s\n" % e)
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

# **products_search_get**
> DataProductListResult products_search_get(q=q, tags=tags, limit=limit, offset=offset)

**Synchronous variant:** `products_search_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.ProductsApi(api_client)
    q = 'q_example' # str | Search query (optional)
    tags = 'tags_example' # str | Comma-separated list of tags to filter by (optional)
    limit = 20 # int | Maximum number of data products to return (optional) (default to 20)
    offset = 0 # int | Number of data products to skip (optional) (default to 0)

    try:
        # Search data products
        api_response = await api_instance.products_search_get(q=q, tags=tags, limit=limit, offset=offset)
        print("The response of ProductsApi->products_search_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProductsApi->products_search_get: %s\n" % e)
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

