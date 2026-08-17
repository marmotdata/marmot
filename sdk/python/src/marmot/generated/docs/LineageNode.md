# LineageNode


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**asset** | [**Asset**](Asset.md) |  | [optional] 
**depth** | **int** |  | [optional] 
**id** | **str** |  | [optional] 
**type** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.lineage_node import LineageNode

# TODO update the JSON string below
json = "{}"
# create an instance of LineageNode from a JSON string
lineage_node_instance = LineageNode.from_json(json)
# print the JSON string representation of the object
print(LineageNode.to_json())

# convert the object into a dict
lineage_node_dict = lineage_node_instance.to_dict()
# create an instance of LineageNode from a dict
lineage_node_from_dict = LineageNode.from_dict(lineage_node_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


