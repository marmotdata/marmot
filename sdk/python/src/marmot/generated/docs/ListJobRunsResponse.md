# ListJobRunsResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**limit** | **int** |  | [optional] 
**offset** | **int** |  | [optional] 
**runs** | [**List[JobRun]**](JobRun.md) |  | [optional] 
**total** | **int** |  | [optional] 

## Example

```python
from marmot.generated.models.list_job_runs_response import ListJobRunsResponse

# TODO update the JSON string below
json = "{}"
# create an instance of ListJobRunsResponse from a JSON string
list_job_runs_response_instance = ListJobRunsResponse.from_json(json)
# print the JSON string representation of the object
print(ListJobRunsResponse.to_json())

# convert the object into a dict
list_job_runs_response_dict = list_job_runs_response_instance.to_dict()
# create an instance of ListJobRunsResponse from a dict
list_job_runs_response_from_dict = ListJobRunsResponse.from_dict(list_job_runs_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


