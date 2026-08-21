# marmot.generated.GlossaryApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**delete_glossary_id**](GlossaryApi.md#delete_glossary_id) | **DELETE** /api/v1/glossary/{id} | Delete glossary term
[**get_glossary_ancestors_id**](GlossaryApi.md#get_glossary_ancestors_id) | **GET** /api/v1/glossary/ancestors/{id} | Get ancestor terms
[**get_glossary_children_id**](GlossaryApi.md#get_glossary_children_id) | **GET** /api/v1/glossary/children/{id} | Get child terms
[**get_glossary_id**](GlossaryApi.md#get_glossary_id) | **GET** /api/v1/glossary/{id} | Get glossary term
[**get_glossary_list**](GlossaryApi.md#get_glossary_list) | **GET** /api/v1/glossary/list | List glossary terms
[**get_glossary_search**](GlossaryApi.md#get_glossary_search) | **GET** /api/v1/glossary/search | Search glossary terms
[**post_glossary**](GlossaryApi.md#post_glossary) | **POST** /api/v1/glossary/ | Create glossary term
[**put_glossary_id**](GlossaryApi.md#put_glossary_id) | **PUT** /api/v1/glossary/{id} | Update glossary term


# **delete_glossary_id**
> Dict[str, str] delete_glossary_id(id)

**Synchronous variant:** `delete_glossary_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete glossary term

Delete a glossary term by its ID

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
    api_instance = marmot.generated.GlossaryApi(api_client)
    id = 'id_example' # str | Glossary Term ID

    try:
        # Delete glossary term
        api_response = await api_instance.delete_glossary_id(id)
        print("The response of GlossaryApi->delete_glossary_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling GlossaryApi->delete_glossary_id: %s\n" % e)
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

# **get_glossary_ancestors_id**
> Dict[str, object] get_glossary_ancestors_id(id)

**Synchronous variant:** `get_glossary_ancestors_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get ancestor terms

Retrieve all ancestor terms of a glossary term (parent chain)

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
    api_instance = marmot.generated.GlossaryApi(api_client)
    id = 'id_example' # str | Term ID

    try:
        # Get ancestor terms
        api_response = await api_instance.get_glossary_ancestors_id(id)
        print("The response of GlossaryApi->get_glossary_ancestors_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling GlossaryApi->get_glossary_ancestors_id: %s\n" % e)
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

# **get_glossary_children_id**
> Dict[str, object] get_glossary_children_id(id)

**Synchronous variant:** `get_glossary_children_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get child terms

Retrieve all child terms of a glossary term

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
    api_instance = marmot.generated.GlossaryApi(api_client)
    id = 'id_example' # str | Parent Term ID

    try:
        # Get child terms
        api_response = await api_instance.get_glossary_children_id(id)
        print("The response of GlossaryApi->get_glossary_children_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling GlossaryApi->get_glossary_children_id: %s\n" % e)
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

# **get_glossary_id**
> GlossaryTerm get_glossary_id(id)

**Synchronous variant:** `get_glossary_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.GlossaryApi(api_client)
    id = 'id_example' # str | Glossary Term ID

    try:
        # Get glossary term
        api_response = await api_instance.get_glossary_id(id)
        print("The response of GlossaryApi->get_glossary_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling GlossaryApi->get_glossary_id: %s\n" % e)
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

# **get_glossary_list**
> GlossaryListResult get_glossary_list(limit=limit, offset=offset)

**Synchronous variant:** `get_glossary_list_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.GlossaryApi(api_client)
    limit = 20 # int | Maximum number of terms to return (optional) (default to 20)
    offset = 0 # int | Number of terms to skip (optional) (default to 0)

    try:
        # List glossary terms
        api_response = await api_instance.get_glossary_list(limit=limit, offset=offset)
        print("The response of GlossaryApi->get_glossary_list:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling GlossaryApi->get_glossary_list: %s\n" % e)
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

# **get_glossary_search**
> GlossaryListResult get_glossary_search(q=q, parent_term_id=parent_term_id, limit=limit, offset=offset)

**Synchronous variant:** `get_glossary_search_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.GlossaryApi(api_client)
    q = 'q_example' # str | Search query (optional)
    parent_term_id = 'parent_term_id_example' # str | Filter by parent term ID (optional)
    limit = 20 # int | Maximum number of terms to return (optional) (default to 20)
    offset = 0 # int | Number of terms to skip (optional) (default to 0)

    try:
        # Search glossary terms
        api_response = await api_instance.get_glossary_search(q=q, parent_term_id=parent_term_id, limit=limit, offset=offset)
        print("The response of GlossaryApi->get_glossary_search:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling GlossaryApi->get_glossary_search: %s\n" % e)
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

# **post_glossary**
> GlossaryTerm post_glossary(create_term_request)

**Synchronous variant:** `post_glossary_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.GlossaryApi(api_client)
    create_term_request = marmot.generated.CreateTermRequest() # CreateTermRequest | Glossary term to create

    try:
        # Create glossary term
        api_response = await api_instance.post_glossary(create_term_request)
        print("The response of GlossaryApi->post_glossary:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling GlossaryApi->post_glossary: %s\n" % e)
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

# **put_glossary_id**
> GlossaryTerm put_glossary_id(id, update_term_request)

**Synchronous variant:** `put_glossary_id_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

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
    api_instance = marmot.generated.GlossaryApi(api_client)
    id = 'id_example' # str | Glossary Term ID
    update_term_request = marmot.generated.UpdateTermRequest() # UpdateTermRequest | Glossary term update data

    try:
        # Update glossary term
        api_response = await api_instance.put_glossary_id(id, update_term_request)
        print("The response of GlossaryApi->put_glossary_id:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling GlossaryApi->put_glossary_id: %s\n" % e)
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

