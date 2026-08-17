# marmot.generated.UsersApi

All URIs are relative to */api/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**users_apikeys_get**](UsersApi.md#users_apikeys_get) | **GET** /users/apikeys | List API keys
[**users_apikeys_id_delete**](UsersApi.md#users_apikeys_id_delete) | **DELETE** /users/apikeys/{id} | Delete API key
[**users_apikeys_post**](UsersApi.md#users_apikeys_post) | **POST** /users/apikeys | Create API key
[**users_get**](UsersApi.md#users_get) | **GET** /users | List users
[**users_id_delete**](UsersApi.md#users_id_delete) | **DELETE** /users/{id} | Delete a user
[**users_id_get**](UsersApi.md#users_id_get) | **GET** /users/{id} | Get a user by ID
[**users_id_put**](UsersApi.md#users_id_put) | **PUT** /users/{id} | Update a user
[**users_login_post**](UsersApi.md#users_login_post) | **POST** /users/login | Login user
[**users_me_get**](UsersApi.md#users_me_get) | **GET** /users/me | Get current user profile
[**users_oauth_link_post**](UsersApi.md#users_oauth_link_post) | **POST** /users/oauth/link | Link OAuth account
[**users_oauth_unlink_id_provider_delete**](UsersApi.md#users_oauth_unlink_id_provider_delete) | **DELETE** /users/oauth/unlink/{id}/{provider} | Unlink OAuth account
[**users_post**](UsersApi.md#users_post) | **POST** /users | Create a new user
[**users_preferences_put**](UsersApi.md#users_preferences_put) | **PUT** /users/preferences | Update user preferences
[**users_update_password_post**](UsersApi.md#users_update_password_post) | **POST** /users/update-password | Update user password


# **users_apikeys_get**
> List[APIKey] users_apikeys_get()

**Synchronous variant:** `users_apikeys_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List API keys

Get all API keys for a user

### Example


```python
import marmot.generated
from marmot.generated.models.api_key import APIKey
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
    api_instance = marmot.generated.UsersApi(api_client)

    try:
        # List API keys
        api_response = await api_instance.users_apikeys_get()
        print("The response of UsersApi->users_apikeys_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling UsersApi->users_apikeys_get: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**List[APIKey]**](APIKey.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **users_apikeys_id_delete**
> users_apikeys_id_delete(id)

**Synchronous variant:** `users_apikeys_id_delete_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete API key

Delete an API key

### Example


```python
import marmot.generated
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
    api_instance = marmot.generated.UsersApi(api_client)
    id = 'id_example' # str | API key ID

    try:
        # Delete API key
        await api_instance.users_apikeys_id_delete(id)
    except Exception as e:
        print("Exception when calling UsersApi->users_apikeys_id_delete: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| API key ID | 

### Return type

void (empty response body)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**204** | No Content |  -  |
**400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **users_apikeys_post**
> APIKey users_apikeys_post(create_api_key_request)

**Synchronous variant:** `users_apikeys_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Create API key

Create a new API key for a user

### Example


```python
import marmot.generated
from marmot.generated.models.api_key import APIKey
from marmot.generated.models.create_api_key_request import CreateAPIKeyRequest
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
    api_instance = marmot.generated.UsersApi(api_client)
    create_api_key_request = marmot.generated.CreateAPIKeyRequest() # CreateAPIKeyRequest | API key creation request

    try:
        # Create API key
        api_response = await api_instance.users_apikeys_post(create_api_key_request)
        print("The response of UsersApi->users_apikeys_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling UsersApi->users_apikeys_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **create_api_key_request** | [**CreateAPIKeyRequest**](CreateAPIKeyRequest.md)| API key creation request | 

### Return type

[**APIKey**](APIKey.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **users_get**
> ListUsersResponse users_get(limit=limit, offset=offset, query=query, role_ids=role_ids, active=active)

**Synchronous variant:** `users_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List users

Get a list of users with optional filtering

### Example


```python
import marmot.generated
from marmot.generated.models.list_users_response import ListUsersResponse
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
    api_instance = marmot.generated.UsersApi(api_client)
    limit = 50 # int | Number of items to return (optional) (default to 50)
    offset = 0 # int | Number of items to skip (optional) (default to 0)
    query = 'query_example' # str | Search query for username or email (optional)
    role_ids = ['role_ids_example'] # List[str] | Filter by role IDs (optional)
    active = True # bool | Filter by active status (optional)

    try:
        # List users
        api_response = await api_instance.users_get(limit=limit, offset=offset, query=query, role_ids=role_ids, active=active)
        print("The response of UsersApi->users_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling UsersApi->users_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **limit** | **int**| Number of items to return | [optional] [default to 50]
 **offset** | **int**| Number of items to skip | [optional] [default to 0]
 **query** | **str**| Search query for username or email | [optional] 
 **role_ids** | [**List[str]**](str.md)| Filter by role IDs | [optional] 
 **active** | **bool**| Filter by active status | [optional] 

### Return type

[**ListUsersResponse**](ListUsersResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **users_id_delete**
> users_id_delete(id)

**Synchronous variant:** `users_id_delete_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete a user

Delete a user from the system

### Example


```python
import marmot.generated
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
    api_instance = marmot.generated.UsersApi(api_client)
    id = 'id_example' # str | User ID

    try:
        # Delete a user
        await api_instance.users_id_delete(id)
    except Exception as e:
        print("Exception when calling UsersApi->users_id_delete: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| User ID | 

### Return type

void (empty response body)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**204** | No Content |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **users_id_get**
> User users_id_get(id)

**Synchronous variant:** `users_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get a user by ID

Get detailed information about a specific user

### Example


```python
import marmot.generated
from marmot.generated.models.user import User
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
    api_instance = marmot.generated.UsersApi(api_client)
    id = 'id_example' # str | User ID

    try:
        # Get a user by ID
        api_response = await api_instance.users_id_get(id)
        print("The response of UsersApi->users_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling UsersApi->users_id_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| User ID | 

### Return type

[**User**](User.md)

### Authorization

No authorization required

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

# **users_id_put**
> User users_id_put(id, update_user_input)

**Synchronous variant:** `users_id_put_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Update a user

Update user information

### Example


```python
import marmot.generated
from marmot.generated.models.update_user_input import UpdateUserInput
from marmot.generated.models.user import User
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
    api_instance = marmot.generated.UsersApi(api_client)
    id = 'id_example' # str | User ID
    update_user_input = marmot.generated.UpdateUserInput() # UpdateUserInput | User update request

    try:
        # Update a user
        api_response = await api_instance.users_id_put(id, update_user_input)
        print("The response of UsersApi->users_id_put:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling UsersApi->users_id_put: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| User ID | 
 **update_user_input** | [**UpdateUserInput**](UpdateUserInput.md)| User update request | 

### Return type

[**User**](User.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**400** | Bad Request |  -  |
**404** | Not Found |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **users_login_post**
> TokenResponse users_login_post(login_request)

**Synchronous variant:** `users_login_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Login user

Authenticate a user with username/email and password

### Example


```python
import marmot.generated
from marmot.generated.models.login_request import LoginRequest
from marmot.generated.models.token_response import TokenResponse
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
    api_instance = marmot.generated.UsersApi(api_client)
    login_request = marmot.generated.LoginRequest() # LoginRequest | Login credentials

    try:
        # Login user
        api_response = await api_instance.users_login_post(login_request)
        print("The response of UsersApi->users_login_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling UsersApi->users_login_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **login_request** | [**LoginRequest**](LoginRequest.md)| Login credentials | 

### Return type

[**TokenResponse**](TokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**400** | Bad Request |  -  |
**401** | Unauthorized |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **users_me_get**
> User users_me_get()

**Synchronous variant:** `users_me_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get current user profile

Get detailed information about the currently authenticated user

### Example

* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.user import User
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

# Configure API key authorization: BearerAuth
configuration.api_key['BearerAuth'] = os.environ["API_KEY"]

# Uncomment below to setup prefix (e.g. Bearer) for API key, if needed
# configuration.api_key_prefix['BearerAuth'] = 'Bearer'

# Enter a context with an instance of the API client
async with marmot.generated.ApiClient(configuration) as api_client:
    # Create an instance of the API class
    api_instance = marmot.generated.UsersApi(api_client)

    try:
        # Get current user profile
        api_response = await api_instance.users_me_get()
        print("The response of UsersApi->users_me_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling UsersApi->users_me_get: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**User**](User.md)

### Authorization

[BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**401** | Unauthorized |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **users_oauth_link_post**
> users_oauth_link_post(o_auth_link_request)

**Synchronous variant:** `users_oauth_link_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Link OAuth account

Link an OAuth account to an existing user

### Example


```python
import marmot.generated
from marmot.generated.models.o_auth_link_request import OAuthLinkRequest
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
    api_instance = marmot.generated.UsersApi(api_client)
    o_auth_link_request = marmot.generated.OAuthLinkRequest() # OAuthLinkRequest | OAuth account link request

    try:
        # Link OAuth account
        await api_instance.users_oauth_link_post(o_auth_link_request)
    except Exception as e:
        print("Exception when calling UsersApi->users_oauth_link_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **o_auth_link_request** | [**OAuthLinkRequest**](OAuthLinkRequest.md)| OAuth account link request | 

### Return type

void (empty response body)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **users_oauth_unlink_id_provider_delete**
> users_oauth_unlink_id_provider_delete(id, provider)

**Synchronous variant:** `users_oauth_unlink_id_provider_delete_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Unlink OAuth account

Unlink an OAuth account from a user

### Example


```python
import marmot.generated
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
    api_instance = marmot.generated.UsersApi(api_client)
    id = 'id_example' # str | User ID
    provider = 'provider_example' # str | OAuth provider

    try:
        # Unlink OAuth account
        await api_instance.users_oauth_unlink_id_provider_delete(id, provider)
    except Exception as e:
        print("Exception when calling UsersApi->users_oauth_unlink_id_provider_delete: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| User ID | 
 **provider** | **str**| OAuth provider | 

### Return type

void (empty response body)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**204** | No Content |  -  |
**400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **users_post**
> User users_post(create_user_input)

**Synchronous variant:** `users_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Create a new user

Create a new user in the system

### Example


```python
import marmot.generated
from marmot.generated.models.create_user_input import CreateUserInput
from marmot.generated.models.user import User
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
    api_instance = marmot.generated.UsersApi(api_client)
    create_user_input = marmot.generated.CreateUserInput() # CreateUserInput | User creation request

    try:
        # Create a new user
        api_response = await api_instance.users_post(create_user_input)
        print("The response of UsersApi->users_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling UsersApi->users_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **create_user_input** | [**CreateUserInput**](CreateUserInput.md)| User creation request | 

### Return type

[**User**](User.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**400** | Bad Request |  -  |
**409** | Conflict |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **users_preferences_put**
> users_preferences_put(request_body)

**Synchronous variant:** `users_preferences_put_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Update user preferences

Update preferences for the current user

### Example


```python
import marmot.generated
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
    api_instance = marmot.generated.UsersApi(api_client)
    request_body = None # Dict[str, object] | User preferences

    try:
        # Update user preferences
        await api_instance.users_preferences_put(request_body)
    except Exception as e:
        print("Exception when calling UsersApi->users_preferences_put: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **request_body** | [**Dict[str, object]**](object.md)| User preferences | 

### Return type

void (empty response body)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **users_update_password_post**
> TokenResponse users_update_password_post(update_password_request)

**Synchronous variant:** `users_update_password_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Update user password

Update current user's password

### Example


```python
import marmot.generated
from marmot.generated.models.token_response import TokenResponse
from marmot.generated.models.update_password_request import UpdatePasswordRequest
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
    api_instance = marmot.generated.UsersApi(api_client)
    update_password_request = marmot.generated.UpdatePasswordRequest() # UpdatePasswordRequest | Password update request

    try:
        # Update user password
        api_response = await api_instance.users_update_password_post(update_password_request)
        print("The response of UsersApi->users_update_password_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling UsersApi->users_update_password_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **update_password_request** | [**UpdatePasswordRequest**](UpdatePasswordRequest.md)| Password update request | 

### Return type

[**TokenResponse**](TokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**400** | Bad Request |  -  |
**401** | Unauthorized |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

