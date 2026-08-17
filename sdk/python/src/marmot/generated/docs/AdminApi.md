# marmot.generated.AdminApi

All URIs are relative to */api/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**admin_search_reindex_get**](AdminApi.md#admin_search_reindex_get) | **GET** /admin/search/reindex | Get reindex status
[**admin_search_reindex_post**](AdminApi.md#admin_search_reindex_post) | **POST** /admin/search/reindex | Start search reindex


# **admin_search_reindex_get**
> ReindexStatusResponse admin_search_reindex_get()

**Synchronous variant:** `admin_search_reindex_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get reindex status

Check whether a search reindex is currently running and whether Elasticsearch is configured.

### Example


```python
import marmot.generated
from marmot.generated.models.reindex_status_response import ReindexStatusResponse
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
    api_instance = marmot.generated.AdminApi(api_client)

    try:
        # Get reindex status
        api_response = await api_instance.admin_search_reindex_get()
        print("The response of AdminApi->admin_search_reindex_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AdminApi->admin_search_reindex_get: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**ReindexStatusResponse**](ReindexStatusResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**401** | Unauthorized |  -  |
**403** | Forbidden |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **admin_search_reindex_post**
> ReindexAcceptedResponse admin_search_reindex_post()

**Synchronous variant:** `admin_search_reindex_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Start search reindex

Trigger a full reindex from PostgreSQL to Elasticsearch. The reindex runs asynchronously in the background. Only one reindex can run at a time.

### Example


```python
import marmot.generated
from marmot.generated.models.reindex_accepted_response import ReindexAcceptedResponse
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
    api_instance = marmot.generated.AdminApi(api_client)

    try:
        # Start search reindex
        api_response = await api_instance.admin_search_reindex_post()
        print("The response of AdminApi->admin_search_reindex_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AdminApi->admin_search_reindex_post: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**ReindexAcceptedResponse**](ReindexAcceptedResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**202** | Accepted |  -  |
**401** | Unauthorized |  -  |
**403** | Forbidden |  -  |
**409** | Conflict |  -  |
**503** | Service Unavailable |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

