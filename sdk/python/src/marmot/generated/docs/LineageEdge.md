# LineageEdge


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**id** | **str** |  | [optional] 
**job_mrn** | **str** |  | [optional] 
**last_seen_at** | **str** |  | [optional] 
**observation_count** | **int** |  | [optional] 
**origin** | **str** |  | [optional] 
**source** | **str** |  | [optional] 
**target** | **str** |  | [optional] 
**type** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.lineage_edge import LineageEdge

# TODO update the JSON string below
json = "{}"
# create an instance of LineageEdge from a JSON string
lineage_edge_instance = LineageEdge.from_json(json)
# print the JSON string representation of the object
print(LineageEdge.to_json())

# convert the object into a dict
lineage_edge_dict = lineage_edge_instance.to_dict()
# create an instance of LineageEdge from a dict
lineage_edge_from_dict = LineageEdge.from_dict(lineage_edge_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


