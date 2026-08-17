# ListSchedulesResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**limit** | **int** |  | [optional] 
**offset** | **int** |  | [optional] 
**schedules** | [**List[Schedule]**](Schedule.md) |  | [optional] 
**total** | **int** |  | [optional] 

## Example

```python
from marmot.generated.models.list_schedules_response import ListSchedulesResponse

# TODO update the JSON string below
json = "{}"
# create an instance of ListSchedulesResponse from a JSON string
list_schedules_response_instance = ListSchedulesResponse.from_json(json)
# print the JSON string representation of the object
print(ListSchedulesResponse.to_json())

# convert the object into a dict
list_schedules_response_dict = list_schedules_response_instance.to_dict()
# create an instance of ListSchedulesResponse from a dict
list_schedules_response_from_dict = ListSchedulesResponse.from_dict(list_schedules_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


