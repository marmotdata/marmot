# marmot.generated.PluginsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**get_plugins**](PluginsApi.md#get_plugins) | **GET** /api/v1/plugins | List registered plugins
[**get_plugins_aws_credentials_status**](PluginsApi.md#get_plugins_aws_credentials_status) | **GET** /api/v1/plugins/aws/credentials/status | Get AWS credential detection status


# **get_plugins**
> ListPluginsResponse get_plugins()

**Synchronous variant:** `get_plugins_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List registered plugins

### Example


```python
import marmot.generated
from marmot.generated.models.list_plugins_response import ListPluginsResponse
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
    api_instance = marmot.generated.PluginsApi(api_client)

    try:
        # List registered plugins
        api_response = await api_instance.get_plugins()
        print("The response of PluginsApi->get_plugins:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling PluginsApi->get_plugins: %s\n" % e)
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

# **get_plugins_aws_credentials_status**
> AWSCredentialStatus get_plugins_aws_credentials_status()

**Synchronous variant:** `get_plugins_aws_credentials_status_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get AWS credential detection status

Detects if AWS credentials are available from environment or config files

### Example


```python
import marmot.generated
from marmot.generated.models.aws_credential_status import AWSCredentialStatus
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
    api_instance = marmot.generated.PluginsApi(api_client)

    try:
        # Get AWS credential detection status
        api_response = await api_instance.get_plugins_aws_credentials_status()
        print("The response of PluginsApi->get_plugins_aws_credentials_status:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling PluginsApi->get_plugins_aws_credentials_status: %s\n" % e)
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

