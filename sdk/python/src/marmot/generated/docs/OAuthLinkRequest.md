# OAuthLinkRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**provider** | **str** |  | 
**provider_user_id** | **str** |  | 
**user_id** | **str** |  | 
**user_info** | **Dict[str, object]** |  | 

## Example

```python
from marmot.generated.models.o_auth_link_request import OAuthLinkRequest

# TODO update the JSON string below
json = "{}"
# create an instance of OAuthLinkRequest from a JSON string
o_auth_link_request_instance = OAuthLinkRequest.from_json(json)
# print the JSON string representation of the object
print(OAuthLinkRequest.to_json())

# convert the object into a dict
o_auth_link_request_dict = o_auth_link_request_instance.to_dict()
# create an instance of OAuthLinkRequest from a dict
o_auth_link_request_from_dict = OAuthLinkRequest.from_dict(o_auth_link_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


