# LineageResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**edges** | [**List[LineageEdge]**](LineageEdge.md) |  | [optional] 
**nodes** | [**List[LineageNode]**](LineageNode.md) |  | [optional] 

## Example

```python
from marmot.generated.models.lineage_response import LineageResponse

# TODO update the JSON string below
json = "{}"
# create an instance of LineageResponse from a JSON string
lineage_response_instance = LineageResponse.from_json(json)
# print the JSON string representation of the object
print(LineageResponse.to_json())

# convert the object into a dict
lineage_response_dict = lineage_response_instance.to_dict()
# create an instance of LineageResponse from a dict
lineage_response_from_dict = LineageResponse.from_dict(lineage_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


