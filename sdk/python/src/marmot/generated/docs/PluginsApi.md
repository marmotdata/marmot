# marmot.generated.PluginsApi

All URIs are relative to */api/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**api_v1_plugins_aws_credentials_status_get**](PluginsApi.md#api_v1_plugins_aws_credentials_status_get) | **GET** /api/v1/plugins/aws/credentials/status | Get AWS credential detection status
[**api_v1_plugins_get**](PluginsApi.md#api_v1_plugins_get) | **GET** /api/v1/plugins | List registered plugins


# **api_v1_plugins_aws_credentials_status_get**
> AWSCredentialStatus api_v1_plugins_aws_credentials_status_get()

**Synchronous variant:** `api_v1_plugins_aws_credentials_status_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get AWS credential detection status

Detects if AWS credentials are available from environment or config files

### Example


```python
import marmot.generated
from marmot.generated.models.aws_credential_status import AWSCredentialStatus
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
    api_instance = marmot.generated.PluginsApi(api_client)

    try:
        # Get AWS credential detection status
        api_response = await api_instance.api_v1_plugins_aws_credentials_status_get()
        print("The response of PluginsApi->api_v1_plugins_aws_credentials_status_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling PluginsApi->api_v1_plugins_aws_credentials_status_get: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**AWSCredentialStatus**](AWSCredentialStatus.md)

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

# **api_v1_plugins_get**
> ListPluginsResponse api_v1_plugins_get()

**Synchronous variant:** `api_v1_plugins_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List registered plugins

### Example


```python
import marmot.generated
from marmot.generated.models.list_plugins_response import ListPluginsResponse
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
    api_instance = marmot.generated.PluginsApi(api_client)

    try:
        # List registered plugins
        api_response = await api_instance.api_v1_plugins_get()
        print("The response of PluginsApi->api_v1_plugins_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling PluginsApi->api_v1_plugins_get: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**ListPluginsResponse**](ListPluginsResponse.md)

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

