# marmot.generated.OwnersApi

All URIs are relative to */api/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**owners_search_get**](OwnersApi.md#owners_search_get) | **GET** /owners/search | Search owners


# **owners_search_get**
> SearchOwnersResponse owners_search_get(q, limit=limit)

**Synchronous variant:** `owners_search_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Search owners

Search for asset owners (users and teams)

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.search_owners_response import SearchOwnersResponse
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
    api_instance = marmot.generated.OwnersApi(api_client)
    q = 'q_example' # str | Search query
    limit = 20 # int | Maximum number of results (optional) (default to 20)

    try:
        # Search owners
        api_response = await api_instance.owners_search_get(q, limit=limit)
        print("The response of OwnersApi->owners_search_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling OwnersApi->owners_search_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **q** | **str**| Search query | 
 **limit** | **int**| Maximum number of results | [optional] [default to 20]

### Return type

[**SearchOwnersResponse**](SearchOwnersResponse.md)

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

