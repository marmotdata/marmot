# LineageResult


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**error** | **str** |  | [optional] 
**source** | **str** |  | [optional] 
**status** | **str** |  | [optional] 
**target** | **str** |  | [optional] 
**type** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.lineage_result import LineageResult

# TODO update the JSON string below
json = "{}"
# create an instance of LineageResult from a JSON string
lineage_result_instance = LineageResult.from_json(json)
# print the JSON string representation of the object
print(LineageResult.to_json())

# convert the object into a dict
lineage_result_dict = lineage_result_instance.to_dict()
# create an instance of LineageResult from a dict
lineage_result_from_dict = LineageResult.from_dict(lineage_result_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


