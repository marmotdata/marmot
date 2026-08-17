# CreateSSOMappingRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**member_role** | **str** |  | [optional] 
**provider** | **str** |  | [optional] 
**sso_group_name** | **str** |  | [optional] 
**team_id** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.create_sso_mapping_request import CreateSSOMappingRequest

# TODO update the JSON string below
json = "{}"
# create an instance of CreateSSOMappingRequest from a JSON string
create_sso_mapping_request_instance = CreateSSOMappingRequest.from_json(json)
# print the JSON string representation of the object
print(CreateSSOMappingRequest.to_json())

# convert the object into a dict
create_sso_mapping_request_dict = create_sso_mapping_request_instance.to_dict()
# create an instance of CreateSSOMappingRequest from a dict
create_sso_mapping_request_from_dict = CreateSSOMappingRequest.from_dict(create_sso_mapping_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


