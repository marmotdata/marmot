# SSOTeamMapping


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**created_at** | **str** |  | [optional] 
**id** | **str** |  | [optional] 
**member_role** | **str** |  | [optional] 
**provider** | **str** |  | [optional] 
**sso_group_name** | **str** |  | [optional] 
**team_id** | **str** |  | [optional] 
**updated_at** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.sso_team_mapping import SSOTeamMapping

# TODO update the JSON string below
json = "{}"
# create an instance of SSOTeamMapping from a JSON string
sso_team_mapping_instance = SSOTeamMapping.from_json(json)
# print the JSON string representation of the object
print(SSOTeamMapping.to_json())

# convert the object into a dict
sso_team_mapping_dict = sso_team_mapping_instance.to_dict()
# create an instance of SSOTeamMapping from a dict
sso_team_mapping_from_dict = SSOTeamMapping.from_dict(sso_team_mapping_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


