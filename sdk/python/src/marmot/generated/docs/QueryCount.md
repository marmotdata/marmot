# QueryCount


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**count** | **int** |  | [optional] 
**query** | **str** |  | [optional] 
**query_type** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.query_count import QueryCount

# TODO update the JSON string below
json = "{}"
# create an instance of QueryCount from a JSON string
query_count_instance = QueryCount.from_json(json)
# print the JSON string representation of the object
print(QueryCount.to_json())

# convert the object into a dict
query_count_dict = query_count_instance.to_dict()
# create an instance of QueryCount from a dict
query_count_from_dict = QueryCount.from_dict(query_count_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


