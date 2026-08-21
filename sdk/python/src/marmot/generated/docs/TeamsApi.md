# marmot.generated.TeamsApi

All URIs are relative to */api/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**teams_get**](TeamsApi.md#teams_get) | **GET** /teams | List teams
[**teams_id_delete**](TeamsApi.md#teams_id_delete) | **DELETE** /teams/{id} | Delete a team
[**teams_id_get**](TeamsApi.md#teams_id_get) | **GET** /teams/{id} | Get a team
[**teams_id_members_get**](TeamsApi.md#teams_id_members_get) | **GET** /teams/{id}/members | List team members
[**teams_id_members_post**](TeamsApi.md#teams_id_members_post) | **POST** /teams/{id}/members | Add a team member
[**teams_id_members_user_id_convert_to_manual_post**](TeamsApi.md#teams_id_members_user_id_convert_to_manual_post) | **POST** /teams/{id}/members/{userId}/convert-to-manual | Convert member to manual
[**teams_id_members_user_id_delete**](TeamsApi.md#teams_id_members_user_id_delete) | **DELETE** /teams/{id}/members/{userId} | Remove a team member
[**teams_id_members_user_id_role_put**](TeamsApi.md#teams_id_members_user_id_role_put) | **PUT** /teams/{id}/members/{userId}/role | Update member role
[**teams_id_put**](TeamsApi.md#teams_id_put) | **PUT** /teams/{id} | Update a team
[**teams_post**](TeamsApi.md#teams_post) | **POST** /teams | Create a team


# **teams_get**
> ListTeamsResponse teams_get(limit=limit, offset=offset)

**Synchronous variant:** `teams_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List teams

Get a paginated list of teams

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.list_teams_response import ListTeamsResponse
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
    api_instance = marmot.generated.TeamsApi(api_client)
    limit = 50 # int | Number of items to return (optional) (default to 50)
    offset = 0 # int | Number of items to skip (optional) (default to 0)

    try:
        # List teams
        api_response = await api_instance.teams_get(limit=limit, offset=offset)
        print("The response of TeamsApi->teams_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling TeamsApi->teams_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **limit** | **int**| Number of items to return | [optional] [default to 50]
 **offset** | **int**| Number of items to skip | [optional] [default to 0]

### Return type

[**ListTeamsResponse**](ListTeamsResponse.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **teams_id_delete**
> MessageResponse teams_id_delete(id)

**Synchronous variant:** `teams_id_delete_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete a team

Delete a team by its ID

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.message_response import MessageResponse
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
    api_instance = marmot.generated.TeamsApi(api_client)
    id = 'id_example' # str | Team ID

    try:
        # Delete a team
        api_response = await api_instance.teams_id_delete(id)
        print("The response of TeamsApi->teams_id_delete:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling TeamsApi->teams_id_delete: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Team ID | 

### Return type

[**MessageResponse**](MessageResponse.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**403** | Forbidden |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **teams_id_get**
> Team teams_id_get(id)

**Synchronous variant:** `teams_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get a team

Get a team by its ID

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.team import Team
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
    api_instance = marmot.generated.TeamsApi(api_client)
    id = 'id_example' # str | Team ID

    try:
        # Get a team
        api_response = await api_instance.teams_id_get(id)
        print("The response of TeamsApi->teams_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling TeamsApi->teams_id_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Team ID | 

### Return type

[**Team**](Team.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **teams_id_members_get**
> ListMembersResponse teams_id_members_get(id)

**Synchronous variant:** `teams_id_members_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List team members

Get the members of a team

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.list_members_response import ListMembersResponse
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
    api_instance = marmot.generated.TeamsApi(api_client)
    id = 'id_example' # str | Team ID

    try:
        # List team members
        api_response = await api_instance.teams_id_members_get(id)
        print("The response of TeamsApi->teams_id_members_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling TeamsApi->teams_id_members_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Team ID | 

### Return type

[**ListMembersResponse**](ListMembersResponse.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **teams_id_members_post**
> MessageResponse teams_id_members_post(id, add_member_request)

**Synchronous variant:** `teams_id_members_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Add a team member

Add a user as a member of a team

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.add_member_request import AddMemberRequest
from marmot.generated.models.message_response import MessageResponse
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
    api_instance = marmot.generated.TeamsApi(api_client)
    id = 'id_example' # str | Team ID
    add_member_request = marmot.generated.AddMemberRequest() # AddMemberRequest | Member addition request

    try:
        # Add a team member
        api_response = await api_instance.teams_id_members_post(id, add_member_request)
        print("The response of TeamsApi->teams_id_members_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling TeamsApi->teams_id_members_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Team ID | 
 **add_member_request** | [**AddMemberRequest**](AddMemberRequest.md)| Member addition request | 

### Return type

[**MessageResponse**](MessageResponse.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**201** | Created |  -  |
**400** | Bad Request |  -  |
**403** | Forbidden |  -  |
**409** | Conflict |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **teams_id_members_user_id_convert_to_manual_post**
> MessageResponse teams_id_members_user_id_convert_to_manual_post(id, user_id)

**Synchronous variant:** `teams_id_members_user_id_convert_to_manual_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Convert member to manual

Convert an SSO-managed team member to a manually managed member

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.message_response import MessageResponse
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
    api_instance = marmot.generated.TeamsApi(api_client)
    id = 'id_example' # str | Team ID
    user_id = 'user_id_example' # str | User ID

    try:
        # Convert member to manual
        api_response = await api_instance.teams_id_members_user_id_convert_to_manual_post(id, user_id)
        print("The response of TeamsApi->teams_id_members_user_id_convert_to_manual_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling TeamsApi->teams_id_members_user_id_convert_to_manual_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Team ID | 
 **user_id** | **str**| User ID | 

### Return type

[**MessageResponse**](MessageResponse.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **teams_id_members_user_id_delete**
> MessageResponse teams_id_members_user_id_delete(id, user_id)

**Synchronous variant:** `teams_id_members_user_id_delete_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Remove a team member

Remove a user from a team

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.message_response import MessageResponse
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
    api_instance = marmot.generated.TeamsApi(api_client)
    id = 'id_example' # str | Team ID
    user_id = 'user_id_example' # str | User ID

    try:
        # Remove a team member
        api_response = await api_instance.teams_id_members_user_id_delete(id, user_id)
        print("The response of TeamsApi->teams_id_members_user_id_delete:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling TeamsApi->teams_id_members_user_id_delete: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Team ID | 
 **user_id** | **str**| User ID | 

### Return type

[**MessageResponse**](MessageResponse.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **teams_id_members_user_id_role_put**
> MessageResponse teams_id_members_user_id_role_put(id, user_id, update_member_role_request)

**Synchronous variant:** `teams_id_members_user_id_role_put_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Update member role

Update the role of a team member

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.message_response import MessageResponse
from marmot.generated.models.update_member_role_request import UpdateMemberRoleRequest
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
    api_instance = marmot.generated.TeamsApi(api_client)
    id = 'id_example' # str | Team ID
    user_id = 'user_id_example' # str | User ID
    update_member_role_request = marmot.generated.UpdateMemberRoleRequest() # UpdateMemberRoleRequest | Role update request

    try:
        # Update member role
        api_response = await api_instance.teams_id_members_user_id_role_put(id, user_id, update_member_role_request)
        print("The response of TeamsApi->teams_id_members_user_id_role_put:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling TeamsApi->teams_id_members_user_id_role_put: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Team ID | 
 **user_id** | **str**| User ID | 
 **update_member_role_request** | [**UpdateMemberRoleRequest**](UpdateMemberRoleRequest.md)| Role update request | 

### Return type

[**MessageResponse**](MessageResponse.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**400** | Bad Request |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **teams_id_put**
> MessageResponse teams_id_put(id, update_team_request)

**Synchronous variant:** `teams_id_put_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Update a team

Update a team's fields by its ID

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.message_response import MessageResponse
from marmot.generated.models.update_team_request import UpdateTeamRequest
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
    api_instance = marmot.generated.TeamsApi(api_client)
    id = 'id_example' # str | Team ID
    update_team_request = marmot.generated.UpdateTeamRequest() # UpdateTeamRequest | Team update request

    try:
        # Update a team
        api_response = await api_instance.teams_id_put(id, update_team_request)
        print("The response of TeamsApi->teams_id_put:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling TeamsApi->teams_id_put: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Team ID | 
 **update_team_request** | [**UpdateTeamRequest**](UpdateTeamRequest.md)| Team update request | 

### Return type

[**MessageResponse**](MessageResponse.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**400** | Bad Request |  -  |
**403** | Forbidden |  -  |
**404** | Not Found |  -  |
**409** | Conflict |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **teams_post**
> Team teams_post(create_team_request)

**Synchronous variant:** `teams_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Create a team

Create a new team

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.create_team_request import CreateTeamRequest
from marmot.generated.models.team import Team
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
    api_instance = marmot.generated.TeamsApi(api_client)
    create_team_request = marmot.generated.CreateTeamRequest() # CreateTeamRequest | Team creation request

    try:
        # Create a team
        api_response = await api_instance.teams_post(create_team_request)
        print("The response of TeamsApi->teams_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling TeamsApi->teams_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **create_team_request** | [**CreateTeamRequest**](CreateTeamRequest.md)| Team creation request | 

### Return type

[**Team**](Team.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**201** | Created |  -  |
**400** | Bad Request |  -  |
**409** | Conflict |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

