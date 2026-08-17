# marmot.generated.PipelinesApi

All URIs are relative to */api/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**pipelines_pipeline_name_delete**](PipelinesApi.md#pipelines_pipeline_name_delete) | **DELETE** /pipelines/{pipelineName} | Destroy pipeline


# **pipelines_pipeline_name_delete**
> DestroyRunResponse pipelines_pipeline_name_delete(pipeline_name)

**Synchronous variant:** `pipelines_pipeline_name_delete_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Destroy pipeline

Delete all resources ever created by a pipeline (across all sources)

### Example


```python
import marmot.generated
from marmot.generated.models.destroy_run_response import DestroyRunResponse
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
    api_instance = marmot.generated.PipelinesApi(api_client)
    pipeline_name = 'pipeline_name_example' # str | Pipeline Name

    try:
        # Destroy pipeline
        api_response = await api_instance.pipelines_pipeline_name_delete(pipeline_name)
        print("The response of PipelinesApi->pipelines_pipeline_name_delete:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling PipelinesApi->pipelines_pipeline_name_delete: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **pipeline_name** | **str**| Pipeline Name | 

### Return type

[**DestroyRunResponse**](DestroyRunResponse.md)

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

