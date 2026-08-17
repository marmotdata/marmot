# ListTeamsResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**limit** | **int** |  | [optional] 
**offset** | **int** |  | [optional] 
**teams** | [**List[Team]**](Team.md) |  | [optional] 
**total** | **int** |  | [optional] 

## Example

```python
from marmot.generated.models.list_teams_response import ListTeamsResponse

# TODO update the JSON string below
json = "{}"
# create an instance of ListTeamsResponse from a JSON string
list_teams_response_instance = ListTeamsResponse.from_json(json)
# print the JSON string representation of the object
print(ListTeamsResponse.to_json())

# convert the object into a dict
list_teams_response_dict = list_teams_response_instance.to_dict()
# create an instance of ListTeamsResponse from a dict
list_teams_response_from_dict = ListTeamsResponse.from_dict(list_teams_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


