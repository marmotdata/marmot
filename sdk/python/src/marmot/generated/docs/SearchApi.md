# marmot.generated.SearchApi

All URIs are relative to */api/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**search_get**](SearchApi.md#search_get) | **GET** /search | Unified search


# **search_get**
> SearchResponse search_get(q, types=types, limit=limit, offset=offset)

**Synchronous variant:** `search_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Unified search

Search across assets, glossary terms, teams, and users

### Example


```python
import marmot.generated
from marmot.generated.models.search_response import SearchResponse
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to /api/v1
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "/api/v1"
)


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
        api_response = await api_instance.search_get(q, types=types, limit=limit, offset=offset)
        print("The response of SearchApi->search_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling SearchApi->search_get: %s\n" % e)
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

No authorization required

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

