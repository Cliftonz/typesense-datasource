import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type EditorMode = 'builder' | 'raw';
export type Format = 'table' | 'timeseries';
export type BucketMode = 'hits' | 'rangeFacet' | 'groupBy';

export interface MyQuery extends DataQuery {
  editorMode: EditorMode;
  collection?: string;
  q?: string;
  queryBy?: string;
  filterBy?: string;
  sortBy?: string;
  facetBy?: string;
  groupBy?: string;
  perPage?: number;
  rawBody?: string;
  format: Format;
  bucketMode?: BucketMode;
  timeField?: string;
  valueField?: string;
  stats?: boolean;
  minInterval?: string;
  maxPages?: number;
  timezone?: string;
}

export const DEFAULT_QUERY: Partial<MyQuery> = {
  editorMode: 'builder',
  q: '*',
  format: 'table',
};

export type TimeUnit = 's' | 'ms';

export interface MyDataSourceOptions extends DataSourceJsonData {
  url?: string;
  timeUnit?: TimeUnit;
  defaultCollection?: string;
  maxPages?: number;
}

export interface MySecureJsonData {
  apiKey?: string;
}
