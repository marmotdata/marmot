# OAuthErrorResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**error** | **str** |  | [optional] 
**error_description** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.o_auth_error_response import OAuthErrorResponse

# TODO update the JSON string below
json = "{}"
# create an instance of OAuthErrorResponse from a JSON string
o_auth_error_response_instance = OAuthErrorResponse.from_json(json)
# print the JSON string representation of the object
print(OAuthErrorResponse.to_json())

# convert the object into a dict
o_auth_error_response_dict = o_auth_error_response_instance.to_dict()
# create an instance of OAuthErrorResponse from a dict
o_auth_error_response_from_dict = OAuthErrorResponse.from_dict(o_auth_error_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


