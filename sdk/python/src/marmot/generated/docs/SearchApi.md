# marmot.generated.SearchApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**get_search**](SearchApi.md#get_search) | **GET** /api/v1/search | Unified search


# **get_search**
> SearchResponse get_search(q, types=types, limit=limit, offset=offset)

**Synchronous variant:** `get_search_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Unified search

Search across assets, glossary terms, teams, and users

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.search_response import SearchResponse
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
    api_instance = marmot.generated.SearchApi(api_client)
    q = 'q_example' # str | Search query
    types = ['types_example'] # List[str] | Filter by result types (asset, glossary, team, user) (optional)
    limit = 20 # int | Limit (optional) (default to 20)
    offset = 0 # int | Offset (optional) (default to 0)

    try:
        # Unified search
        api_response = await api_instance.get_search(q, types=types, limit=limit, offset=offset)
        print("The response of SearchApi->get_search:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling SearchApi->get_search: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **q** | **str**| Search query | 
 **types** | [**List[str]**](str.md)| Filter by result types (asset, glossary, team, user) | [optional] 
 **limit** | **int**| Limit | [optional] [default to 20]
 **offset** | **int**| Offset | [optional] [default to 0]

### Return type

[**SearchResponse**](SearchResponse.md)

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

