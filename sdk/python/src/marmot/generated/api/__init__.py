# flake8: noqa

if __import__("typing").TYPE_CHECKING:
    # import apis into api package
    from marmot.generated.api.admin_api import AdminApi
    from marmot.generated.api.agents_api import AgentsApi
    from marmot.generated.api.asset_rules_api import AssetRulesApi
    from marmot.generated.api.assets_api import AssetsApi
    from marmot.generated.api.auth_api import AuthApi
    from marmot.generated.api.glossary_api import GlossaryApi
    from marmot.generated.api.ingestion_api import IngestionApi
    from marmot.generated.api.lineage_api import LineageApi
    from marmot.generated.api.metrics_api import MetricsApi
    from marmot.generated.api.owners_api import OwnersApi
    from marmot.generated.api.pipelines_api import PipelinesApi
    from marmot.generated.api.plugins_api import PluginsApi
    from marmot.generated.api.products_api import ProductsApi
    from marmot.generated.api.roles_api import RolesApi
    from marmot.generated.api.runs_api import RunsApi
    from marmot.generated.api.search_api import SearchApi
    from marmot.generated.api.service_accounts_api import ServiceAccountsApi
    from marmot.generated.api.sso_api import SsoApi
    from marmot.generated.api.teams_api import TeamsApi
    from marmot.generated.api.ui_api import UiApi
    from marmot.generated.api.users_api import UsersApi

else:
    from lazy_imports import LazyModule, as_package, load

    load(
        LazyModule(
            *as_package(__file__),
            """# import apis into api package
from marmot.generated.api.admin_api import AdminApi
from marmot.generated.api.agents_api import AgentsApi
from marmot.generated.api.asset_rules_api import AssetRulesApi
from marmot.generated.api.assets_api import AssetsApi
from marmot.generated.api.auth_api import AuthApi
from marmot.generated.api.glossary_api import GlossaryApi
from marmot.generated.api.ingestion_api import IngestionApi
from marmot.generated.api.lineage_api import LineageApi
from marmot.generated.api.metrics_api import MetricsApi
from marmot.generated.api.owners_api import OwnersApi
from marmot.generated.api.pipelines_api import PipelinesApi
from marmot.generated.api.plugins_api import PluginsApi
from marmot.generated.api.products_api import ProductsApi
from marmot.generated.api.roles_api import RolesApi
from marmot.generated.api.runs_api import RunsApi
from marmot.generated.api.search_api import SearchApi
from marmot.generated.api.service_accounts_api import ServiceAccountsApi
from marmot.generated.api.sso_api import SsoApi
from marmot.generated.api.teams_api import TeamsApi
from marmot.generated.api.ui_api import UiApi
from marmot.generated.api.users_api import UsersApi

""",
            name=__name__,
            doc=__doc__,
        )
    )
