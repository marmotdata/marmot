# CreateLineageRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**job_mrn** | **str** |  | [optional] 
**source** | **str** |  | [optional] 
**target** | **str** |  | [optional] 
**type** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.create_lineage_request import CreateLineageRequest

# TODO update the JSON string below
json = "{}"
# create an instance of CreateLineageRequest from a JSON string
create_lineage_request_instance = CreateLineageRequest.from_json(json)
# print the JSON string representation of the object
print(CreateLineageRequest.to_json())

# convert the object into a dict
create_lineage_request_dict = create_lineage_request_instance.to_dict()
# create an instance of CreateLineageRequest from a dict
create_lineage_request_from_dict = CreateLineageRequest.from_dict(create_lineage_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


