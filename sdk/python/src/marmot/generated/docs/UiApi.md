# marmot.generated.UiApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**get_ui_config**](UiApi.md#get_ui_config) | **GET** /api/v1/ui/config | Get UI configuration


# **get_ui_config**
> UIConfigResponse get_ui_config()

**Synchronous variant:** `get_ui_config_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get UI configuration

Get UI configuration including banner settings

### Example


```python
import marmot.generated
from marmot.generated.models.ui_config_response import UIConfigResponse
from marmot.generated.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to http://localhost
# See configuration.py for a list of all supported configuration parameters.
configuration = marmot.generated.Configuration(
    host = "http://localhost"
)


# Enter a context with an instance of the API client
async with marmot.generated.ApiClient(configuration) as api_client:
    # Create an instance of the API class
    api_instance = marmot.generated.UiApi(api_client)

    try:
        # Get UI configuration
        api_response = await api_instance.get_ui_config()
        print("The response of UiApi->get_ui_config:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling UiApi->get_ui_config: %s\n" % e)
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

