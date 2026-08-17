# JobRun


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**assets_created** | **int** |  | [optional] 
**assets_deleted** | **int** |  | [optional] 
**assets_updated** | **int** |  | [optional] 
**claimed_at** | **str** |  | [optional] 
**claimed_by** | **str** |  | [optional] 
**config** | **Dict[str, object]** |  | [optional] 
**created_at** | **str** |  | [optional] 
**created_by** | **str** |  | [optional] 
**documentation_added** | **int** |  | [optional] 
**error_message** | **str** |  | [optional] 
**finished_at** | **str** |  | [optional] 
**id** | **str** |  | [optional] 
**lineage_created** | **int** |  | [optional] 
**log** | **str** |  | [optional] 
**pipeline_name** | **str** |  | [optional] 
**plugin_run_id** | **str** |  | [optional] 
**run_id** | **str** |  | [optional] 
**schedule_id** | **str** |  | [optional] 
**source_name** | **str** |  | [optional] 
**started_at** | **str** |  | [optional] 
**status** | **str** |  | [optional] 
**updated_at** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.job_run import JobRun

# TODO update the JSON string below
json = "{}"
# create an instance of JobRun from a JSON string
job_run_instance = JobRun.from_json(json)
# print the JSON string representation of the object
print(JobRun.to_json())

# convert the object into a dict
job_run_dict = job_run_instance.to_dict()
# create an instance of JobRun from a dict
job_run_from_dict = JobRun.from_dict(job_run_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


