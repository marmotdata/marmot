# TeamOwner


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**email** | **str** |  | [optional] 
**id** | **str** |  | [optional] 
**name** | **str** |  | [optional] 
**profile_picture** | **str** |  | [optional] 
**type** | **str** |  | [optional] 
**username** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.team_owner import TeamOwner

# TODO update the JSON string below
json = "{}"
# create an instance of TeamOwner from a JSON string
team_owner_instance = TeamOwner.from_json(json)
# print the JSON string representation of the object
print(TeamOwner.to_json())

# convert the object into a dict
team_owner_dict = team_owner_instance.to_dict()
# create an instance of TeamOwner from a dict
team_owner_from_dict = TeamOwner.from_dict(team_owner_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


