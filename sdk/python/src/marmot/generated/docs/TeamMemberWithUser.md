# TeamMemberWithUser


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**email** | **str** |  | [optional] 
**id** | **str** |  | [optional] 
**joined_at** | **str** |  | [optional] 
**name** | **str** |  | [optional] 
**profile_picture** | **str** |  | [optional] 
**role** | **str** |  | [optional] 
**source** | **str** |  | [optional] 
**sso_provider** | **str** |  | [optional] 
**team_id** | **str** |  | [optional] 
**user_id** | **str** |  | [optional] 
**username** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.team_member_with_user import TeamMemberWithUser

# TODO update the JSON string below
json = "{}"
# create an instance of TeamMemberWithUser from a JSON string
team_member_with_user_instance = TeamMemberWithUser.from_json(json)
# print the JSON string representation of the object
print(TeamMemberWithUser.to_json())

# convert the object into a dict
team_member_with_user_dict = team_member_with_user_instance.to_dict()
# create an instance of TeamMemberWithUser from a dict
team_member_with_user_from_dict = TeamMemberWithUser.from_dict(team_member_with_user_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


