# LineageRun


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**facets** | **Dict[str, object]** |  | [optional] 
**run_id** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.lineage_run import LineageRun

# TODO update the JSON string below
json = "{}"
# create an instance of LineageRun from a JSON string
lineage_run_instance = LineageRun.from_json(json)
# print the JSON string representation of the object
print(LineageRun.to_json())

# convert the object into a dict
lineage_run_dict = lineage_run_instance.to_dict()
# create an instance of LineageRun from a dict
lineage_run_from_dict = LineageRun.from_dict(lineage_run_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


