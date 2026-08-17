# marmot.generated.UiApi

All URIs are relative to */api/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**ui_config_get**](UiApi.md#ui_config_get) | **GET** /ui/config | Get UI configuration


# **ui_config_get**
> UIConfigResponse ui_config_get()

**Synchronous variant:** `ui_config_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get UI configuration

Get UI configuration including banner settings

### Example


```python
import marmot.generated
from marmot.generated.models.ui_config_response import UIConfigResponse
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
    api_instance = marmot.generated.UiApi(api_client)

    try:
        # Get UI configuration
        api_response = await api_instance.ui_config_get()
        print("The response of UiApi->ui_config_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling UiApi->ui_config_get: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**UIConfigResponse**](UIConfigResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

