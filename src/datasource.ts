import { CoreApp, DataQueryRequest, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { MyQuery, MyDataSourceOptions, DEFAULT_QUERY } from './types';

const templatedKeys = [
  'collection',
  'q',
  'queryBy',
  'filterBy',
  'sortBy',
  'facetBy',
  'groupBy',
  'timeField',
  'valueField',
  'minInterval',
] as const;

const ianaTimezone = (timezone: string) => {
  if (timezone === 'utc') {
    return 'UTC';
  }
  if (timezone === 'browser' || timezone === '') {
    return Intl.DateTimeFormat().resolvedOptions().timeZone;
  }
  return timezone;
};

export class DataSource extends DataSourceWithBackend<MyQuery, MyDataSourceOptions> {
  getDefaultQuery(_: CoreApp): Partial<MyQuery> {
    return { ...DEFAULT_QUERY, collection: this.datasourceInstanceSettings.jsonData.defaultCollection };
  }

  applyTemplateVariables(query: MyQuery, scopedVars: ScopedVars): MyQuery {
    const replaced = templatedKeys.map((key) => [key, getTemplateSrv().replace(query[key], scopedVars)]);
    return {
      ...query,
      ...Object.fromEntries(replaced),
      rawBody: getTemplateSrv().replace(query.rawBody, scopedVars, 'json'),
    };
  }

  filterQuery(query: MyQuery): boolean {
    return query.editorMode === 'raw' ? !!query.rawBody?.trim() : !!query.collection?.trim();
  }

  query(request: DataQueryRequest<MyQuery>) {
    const timezone = ianaTimezone(request.timezone);
    return super.query({ ...request, targets: request.targets.map((target) => ({ ...target, timezone })) });
  }
}
