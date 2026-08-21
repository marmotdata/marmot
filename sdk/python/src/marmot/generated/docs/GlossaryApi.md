# marmot.generated.GlossaryApi

All URIs are relative to */api/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**glossary_ancestors_id_get**](GlossaryApi.md#glossary_ancestors_id_get) | **GET** /glossary/ancestors/{id} | Get ancestor terms
[**glossary_children_id_get**](GlossaryApi.md#glossary_children_id_get) | **GET** /glossary/children/{id} | Get child terms
[**glossary_id_delete**](GlossaryApi.md#glossary_id_delete) | **DELETE** /glossary/{id} | Delete glossary term
[**glossary_id_get**](GlossaryApi.md#glossary_id_get) | **GET** /glossary/{id} | Get glossary term
[**glossary_id_put**](GlossaryApi.md#glossary_id_put) | **PUT** /glossary/{id} | Update glossary term
[**glossary_list_get**](GlossaryApi.md#glossary_list_get) | **GET** /glossary/list | List glossary terms
[**glossary_post**](GlossaryApi.md#glossary_post) | **POST** /glossary/ | Create glossary term
[**glossary_search_get**](GlossaryApi.md#glossary_search_get) | **GET** /glossary/search | Search glossary terms


# **glossary_ancestors_id_get**
> Dict[str, object] glossary_ancestors_id_get(id)

**Synchronous variant:** `glossary_ancestors_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get ancestor terms

Retrieve all ancestor terms of a glossary term (parent chain)

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
    api_instance = marmot.generated.GlossaryApi(api_client)
    id = 'id_example' # str | Term ID

    try:
        # Get ancestor terms
        api_response = await api_instance.glossary_ancestors_id_get(id)
        print("The response of GlossaryApi->glossary_ancestors_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling GlossaryApi->glossary_ancestors_id_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Term ID | 

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
**400** | Bad Request |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **glossary_children_id_get**
> Dict[str, object] glossary_children_id_get(id)

**Synchronous variant:** `glossary_children_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get child terms

Retrieve all child terms of a glossary term

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
    api_instance = marmot.generated.GlossaryApi(api_client)
    id = 'id_example' # str | Parent Term ID

    try:
        # Get child terms
        api_response = await api_instance.glossary_children_id_get(id)
        print("The response of GlossaryApi->glossary_children_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling GlossaryApi->glossary_children_id_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Parent Term ID | 

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
**400** | Bad Request |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **glossary_id_delete**
> Dict[str, str] glossary_id_delete(id)

**Synchronous variant:** `glossary_id_delete_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete glossary term

Delete a glossary term by its ID

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
    api_instance = marmot.generated.GlossaryApi(api_client)
    id = 'id_example' # str | Glossary Term ID

    try:
        # Delete glossary term
        api_response = await api_instance.glossary_id_delete(id)
        print("The response of GlossaryApi->glossary_id_delete:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling GlossaryApi->glossary_id_delete: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Glossary Term ID | 

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

# **glossary_id_get**
> GlossaryTerm glossary_id_get(id)

**Synchronous variant:** `glossary_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get glossary term

Retrieve a glossary term by its ID

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.glossary_term import GlossaryTerm
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
    api_instance = marmot.generated.GlossaryApi(api_client)
    id = 'id_example' # str | Glossary Term ID

    try:
        # Get glossary term
        api_response = await api_instance.glossary_id_get(id)
        print("The response of GlossaryApi->glossary_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling GlossaryApi->glossary_id_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Glossary Term ID | 

### Return type

[**GlossaryTerm**](GlossaryTerm.md)

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

# **glossary_id_put**
> GlossaryTerm glossary_id_put(id, update_term_request)

**Synchronous variant:** `glossary_id_put_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Update glossary term

Update an existing glossary term by its ID

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.glossary_term import GlossaryTerm
from marmot.generated.models.update_term_request import UpdateTermRequest
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
    api_instance = marmot.generated.GlossaryApi(api_client)
    id = 'id_example' # str | Glossary Term ID
    update_term_request = marmot.generated.UpdateTermRequest() # UpdateTermRequest | Glossary term update data

    try:
        # Update glossary term
        api_response = await api_instance.glossary_id_put(id, update_term_request)
        print("The response of GlossaryApi->glossary_id_put:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling GlossaryApi->glossary_id_put: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Glossary Term ID | 
 **update_term_request** | [**UpdateTermRequest**](UpdateTermRequest.md)| Glossary term update data | 

### Return type

[**GlossaryTerm**](GlossaryTerm.md)

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

# **glossary_list_get**
> GlossaryListResult glossary_list_get(limit=limit, offset=offset)

**Synchronous variant:** `glossary_list_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List glossary terms

Retrieve a paginated list of all glossary terms

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.glossary_list_result import GlossaryListResult
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
    api_instance = marmot.generated.GlossaryApi(api_client)
    limit = 20 # int | Maximum number of terms to return (optional) (default to 20)
    offset = 0 # int | Number of terms to skip (optional) (default to 0)

    try:
        # List glossary terms
        api_response = await api_instance.glossary_list_get(limit=limit, offset=offset)
        print("The response of GlossaryApi->glossary_list_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling GlossaryApi->glossary_list_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **limit** | **int**| Maximum number of terms to return | [optional] [default to 20]
 **offset** | **int**| Number of terms to skip | [optional] [default to 0]

### Return type

[**GlossaryListResult**](GlossaryListResult.md)

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

# **glossary_post**
> GlossaryTerm glossary_post(create_term_request)

**Synchronous variant:** `glossary_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Create glossary term

Create a new glossary term with name, definition, and optional metadata

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.create_term_request import CreateTermRequest
from marmot.generated.models.glossary_term import GlossaryTerm
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
    api_instance = marmot.generated.GlossaryApi(api_client)
    create_term_request = marmot.generated.CreateTermRequest() # CreateTermRequest | Glossary term to create

    try:
        # Create glossary term
        api_response = await api_instance.glossary_post(create_term_request)
        print("The response of GlossaryApi->glossary_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling GlossaryApi->glossary_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **create_term_request** | [**CreateTermRequest**](CreateTermRequest.md)| Glossary term to create | 

### Return type

[**GlossaryTerm**](GlossaryTerm.md)

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

# **glossary_search_get**
> GlossaryListResult glossary_search_get(q=q, parent_term_id=parent_term_id, limit=limit, offset=offset)

**Synchronous variant:** `glossary_search_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Search glossary terms

Search for glossary terms by query string and filters

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.glossary_list_result import GlossaryListResult
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
    api_instance = marmot.generated.GlossaryApi(api_client)
    q = 'q_example' # str | Search query (optional)
    parent_term_id = 'parent_term_id_example' # str | Filter by parent term ID (optional)
    limit = 20 # int | Maximum number of terms to return (optional) (default to 20)
    offset = 0 # int | Number of terms to skip (optional) (default to 0)

    try:
        # Search glossary terms
        api_response = await api_instance.glossary_search_get(q=q, parent_term_id=parent_term_id, limit=limit, offset=offset)
        print("The response of GlossaryApi->glossary_search_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling GlossaryApi->glossary_search_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **q** | **str**| Search query | [optional] 
 **parent_term_id** | **str**| Filter by parent term ID | [optional] 
 **limit** | **int**| Maximum number of terms to return | [optional] [default to 20]
 **offset** | **int**| Number of terms to skip | [optional] [default to 0]

### Return type

[**GlossaryListResult**](GlossaryListResult.md)

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

