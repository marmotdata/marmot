# GlossaryOwner


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**email** | **str** |  | [optional] 
**id** | **str** |  | [optional] 
**name** | **str** |  | [optional] 
**profile_picture** | **str** |  | [optional] 
**type** | **str** | \&quot;user\&quot; or \&quot;team\&quot; | [optional] 
**username** | **str** | Only for user owners | [optional] 

## Example

```python
from marmot.generated.models.glossary_owner import GlossaryOwner

# TODO update the JSON string below
json = "{}"
# create an instance of GlossaryOwner from a JSON string
glossary_owner_instance = GlossaryOwner.from_json(json)
# print the JSON string representation of the object
print(GlossaryOwner.to_json())

# convert the object into a dict
glossary_owner_dict = glossary_owner_instance.to_dict()
# create an instance of GlossaryOwner from a dict
glossary_owner_from_dict = GlossaryOwner.from_dict(glossary_owner_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


