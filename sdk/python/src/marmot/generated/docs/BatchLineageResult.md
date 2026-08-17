# BatchLineageResult


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**edge** | [**LineageEdge**](LineageEdge.md) |  | [optional] 
**status** | **str** | \&quot;created\&quot;, \&quot;duplicate\&quot;, or \&quot;existing\&quot; | [optional] 

## Example

```python
from marmot.generated.models.batch_lineage_result import BatchLineageResult

# TODO update the JSON string below
json = "{}"
# create an instance of BatchLineageResult from a JSON string
batch_lineage_result_instance = BatchLineageResult.from_json(json)
# print the JSON string representation of the object
print(BatchLineageResult.to_json())

# convert the object into a dict
batch_lineage_result_dict = batch_lineage_result_instance.to_dict()
# create an instance of BatchLineageResult from a dict
batch_lineage_result_from_dict = BatchLineageResult.from_dict(batch_lineage_result_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


