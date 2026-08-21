# marmot.generated.RolesApi

All URIs are relative to */api/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**permissions_get**](RolesApi.md#permissions_get) | **GET** /permissions | List all permissions
[**roles_get**](RolesApi.md#roles_get) | **GET** /roles | List roles
[**roles_id_delete**](RolesApi.md#roles_id_delete) | **DELETE** /roles/{id} | Delete a role
[**roles_id_get**](RolesApi.md#roles_id_get) | **GET** /roles/{id} | Get a role
[**roles_id_patch**](RolesApi.md#roles_id_patch) | **PATCH** /roles/{id} | Update a role
[**roles_id_permissions_post**](RolesApi.md#roles_id_permissions_post) | **POST** /roles/{id}/permissions | Replace role permissions
[**roles_post**](RolesApi.md#roles_post) | **POST** /roles | Create a role


# **permissions_get**
> List[RolePermission] permissions_get()

**Synchronous variant:** `permissions_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List all permissions

List all defined permissions grouped by resource type

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.role_permission import RolePermission
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
    api_instance = marmot.generated.RolesApi(api_client)

    try:
        # List all permissions
        api_response = await api_instance.permissions_get()
        print("The response of RolesApi->permissions_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling RolesApi->permissions_get: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**List[RolePermission]**](RolePermission.md)

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

# **roles_get**
> List[Role] roles_get()

**Synchronous variant:** `roles_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

List roles

List all active roles with user counts and permissions

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.role import Role
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
    api_instance = marmot.generated.RolesApi(api_client)

    try:
        # List roles
        api_response = await api_instance.roles_get()
        print("The response of RolesApi->roles_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling RolesApi->roles_get: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**List[Role]**](Role.md)

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

# **roles_id_delete**
> roles_id_delete(id)

**Synchronous variant:** `roles_id_delete_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Delete a role

Soft-delete a role. Fails if the role is a system role or has active user assignments.

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
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
    api_instance = marmot.generated.RolesApi(api_client)
    id = 'id_example' # str | Role ID

    try:
        # Delete a role
        await api_instance.roles_id_delete(id)
    except Exception as e:
        print("Exception when calling RolesApi->roles_id_delete: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Role ID | 

### Return type

void (empty response body)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth), [BearerAuth](../README.md#BearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**204** | No Content |  -  |
**404** | Not Found |  -  |
**422** | Unprocessable Entity |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **roles_id_get**
> Role roles_id_get(id)

**Synchronous variant:** `roles_id_get_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Get a role

Get a role by ID with its permissions

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.role import Role
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
    api_instance = marmot.generated.RolesApi(api_client)
    id = 'id_example' # str | Role ID

    try:
        # Get a role
        api_response = await api_instance.roles_id_get(id)
        print("The response of RolesApi->roles_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling RolesApi->roles_id_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Role ID | 

### Return type

[**Role**](Role.md)

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

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **roles_id_patch**
> Role roles_id_patch(id, v1_roles_update_role_request)

**Synchronous variant:** `roles_id_patch_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Update a role

Update a role's name or description. System roles cannot be renamed.

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.role import Role
from marmot.generated.models.v1_roles_update_role_request import V1RolesUpdateRoleRequest
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
    api_instance = marmot.generated.RolesApi(api_client)
    id = 'id_example' # str | Role ID
    v1_roles_update_role_request = marmot.generated.V1RolesUpdateRoleRequest() # V1RolesUpdateRoleRequest | Role update request

    try:
        # Update a role
        api_response = await api_instance.roles_id_patch(id, v1_roles_update_role_request)
        print("The response of RolesApi->roles_id_patch:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling RolesApi->roles_id_patch: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Role ID | 
 **v1_roles_update_role_request** | [**V1RolesUpdateRoleRequest**](V1RolesUpdateRoleRequest.md)| Role update request | 

### Return type

[**Role**](Role.md)

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
**422** | Unprocessable Entity |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **roles_id_permissions_post**
> Role roles_id_permissions_post(id, v1_roles_replace_permissions_request)

**Synchronous variant:** `roles_id_permissions_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Replace role permissions

Atomically replace all permissions on a role. System roles enforce a minimum permission floor.

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.role import Role
from marmot.generated.models.v1_roles_replace_permissions_request import V1RolesReplacePermissionsRequest
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
    api_instance = marmot.generated.RolesApi(api_client)
    id = 'id_example' # str | Role ID
    v1_roles_replace_permissions_request = marmot.generated.V1RolesReplacePermissionsRequest() # V1RolesReplacePermissionsRequest | Permission IDs to assign

    try:
        # Replace role permissions
        api_response = await api_instance.roles_id_permissions_post(id, v1_roles_replace_permissions_request)
        print("The response of RolesApi->roles_id_permissions_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling RolesApi->roles_id_permissions_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Role ID | 
 **v1_roles_replace_permissions_request** | [**V1RolesReplacePermissionsRequest**](V1RolesReplacePermissionsRequest.md)| Permission IDs to assign | 

### Return type

[**Role**](Role.md)

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
**422** | Unprocessable Entity |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **roles_post**
> Role roles_post(v1_roles_create_role_request)

**Synchronous variant:** `roles_post_sync(...)` — same parameters and return type, but blocks until completion instead of requiring `await`.

Create a role

Create a new role with optional initial permissions

### Example

* Api Key Authentication (ApiKeyAuth):
* Api Key Authentication (BearerAuth):

```python
import marmot.generated
from marmot.generated.models.role import Role
from marmot.generated.models.v1_roles_create_role_request import V1RolesCreateRoleRequest
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
    api_instance = marmot.generated.RolesApi(api_client)
    v1_roles_create_role_request = marmot.generated.V1RolesCreateRoleRequest() # V1RolesCreateRoleRequest | Role creation request

    try:
        # Create a role
        api_response = await api_instance.roles_post(v1_roles_create_role_request)
        print("The response of RolesApi->roles_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling RolesApi->roles_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **v1_roles_create_role_request** | [**V1RolesCreateRoleRequest**](V1RolesCreateRoleRequest.md)| Role creation request | 

### Return type

[**Role**](Role.md)

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
**409** | Conflict |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

