# UpdateSSOMappingRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**member_role** | **str** |  | [optional] 
**team_id** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.update_sso_mapping_request import UpdateSSOMappingRequest

# TODO update the JSON string below
json = "{}"
# create an instance of UpdateSSOMappingRequest from a JSON string
update_sso_mapping_request_instance = UpdateSSOMappingRequest.from_json(json)
# print the JSON string representation of the object
print(UpdateSSOMappingRequest.to_json())

# convert the object into a dict
update_sso_mapping_request_dict = update_sso_mapping_request_instance.to_dict()
# create an instance of UpdateSSOMappingRequest from a dict
update_sso_mapping_request_from_dict = UpdateSSOMappingRequest.from_dict(update_sso_mapping_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


