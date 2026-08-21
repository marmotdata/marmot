# marmot.generated.AgentsApi

All URIs are relative to */api/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**agents_asset_id_activity_get**](AgentsApi.md#agents_asset_id_activity_get) | **GET** /agents/{asset_id}/activity | Agent activity
[**agents_asset_id_runs_get**](AgentsApi.md#agents_asset_id_runs_get) | **GET** /agents/{asset_id}/runs | List agent runs
[**agents_asset_id_stats_get**](AgentsApi.md#agents_asset_id_stats_get) | **GET** /agents/{asset_id}/stats | Agent stats
[**agents_runs_post**](AgentsApi.md#agents_runs_post) | **POST** /agents/runs | Record agent run


# **agents_asset_id_activity_get**
> ActivityResponse agents_asset_id_activity_get(asset_id, period=period)

**Synchronous variant:** `agents_asset_id_activity_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Agent activity

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.activity_response import ActivityResponse
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
    api_instance = marmot.generated.AgentsApi(api_client)
    asset_id = 'asset_id_example' # str | Agent asset id
    period = 'period_example' # str | Lookback window (e.g. 24h, 7d). Default 24h. (optional)

    try:
        # Agent activity
        api_response = await api_instance.agents_asset_id_activity_get(asset_id, period=period)
        print("The response of AgentsApi->agents_asset_id_activity_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AgentsApi->agents_asset_id_activity_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **asset_id** | **str**| Agent asset id | 
 **period** | **str**| Lookback window (e.g. 24h, 7d). Default 24h. | [optional] 

### Return type

[**ActivityResponse**](ActivityResponse.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **agents_asset_id_runs_get**
> RunsResponse agents_asset_id_runs_get(asset_id, period=period, limit=limit)

**Synchronous variant:** `agents_asset_id_runs_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List agent runs

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.runs_response import RunsResponse
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
    api_instance = marmot.generated.AgentsApi(api_client)
    asset_id = 'asset_id_example' # str | Agent asset id
    period = 'period_example' # str | Lookback window (e.g. 24h, 7d). Default 24h. (optional)
    limit = 56 # int | Max number of runs to return (optional)

    try:
        # List agent runs
        api_response = await api_instance.agents_asset_id_runs_get(asset_id, period=period, limit=limit)
        print("The response of AgentsApi->agents_asset_id_runs_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AgentsApi->agents_asset_id_runs_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **asset_id** | **str**| Agent asset id | 
 **period** | **str**| Lookback window (e.g. 24h, 7d). Default 24h. | [optional] 
 **limit** | **int**| Max number of runs to return | [optional] 

### Return type

[**RunsResponse**](RunsResponse.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **agents_asset_id_stats_get**
> Stats agents_asset_id_stats_get(asset_id, period=period)

**Synchronous variant:** `agents_asset_id_stats_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Agent stats

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.stats import Stats
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
    api_instance = marmot.generated.AgentsApi(api_client)
    asset_id = 'asset_id_example' # str | Agent asset id
    period = 'period_example' # str | Lookback window (e.g. 24h, 7d). Default 24h. (optional)

    try:
        # Agent stats
        api_response = await api_instance.agents_asset_id_stats_get(asset_id, period=period)
        print("The response of AgentsApi->agents_asset_id_stats_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AgentsApi->agents_asset_id_stats_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **asset_id** | **str**| Agent asset id | 
 **period** | **str**| Lookback window (e.g. 24h, 7d). Default 24h. | [optional] 

### Return type

[**Stats**](Stats.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **agents_runs_post**
> AgentRun agents_runs_post(record_run_request)

**Synchronous variant:** `agents_runs_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Record agent run

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.agent_run import AgentRun
from marmot.generated.models.record_run_request import RecordRunRequest
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
    api_instance = marmot.generated.AgentsApi(api_client)
    record_run_request = marmot.generated.RecordRunRequest() # RecordRunRequest | Agent run record

    try:
        # Record agent run
        api_response = await api_instance.agents_runs_post(record_run_request)
        print("The response of AgentsApi->agents_runs_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AgentsApi->agents_runs_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **record_run_request** | [**RecordRunRequest**](RecordRunRequest.md)| Agent run record | 

### Return type

[**AgentRun**](AgentRun.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**201** | Created |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

