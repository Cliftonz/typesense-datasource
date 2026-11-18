import { lastValueFrom, of } from 'rxjs';
import { CoreApp, DataQueryRequest, DataSourceInstanceSettings, dateTime } from '@grafana/data';
import { BackendSrv, BackendSrvRequest, setBackendSrv, setTemplateSrv, TemplateSrv } from '@grafana/runtime';
import { DataSource } from './datasource';
import { MyDataSourceOptions, MyQuery } from './types';

const newDataSource = (jsonData: MyDataSourceOptions = {}) =>
  new DataSource({
    uid: 'ts',
    type: 'cliftonz-typesense-datasource',
    jsonData,
  } as unknown as DataSourceInstanceSettings<MyDataSourceOptions>);

beforeEach(() => {
  setTemplateSrv({
    replace: (target?: string, _?: unknown, format?: string) =>
      (target ?? '').replace('$var', format === 'json' ? '["a","b"]' : 'resolved'),
  } as TemplateSrv);
});

const sentQueries = async (timezone: string, targets: MyQuery[]): Promise<MyQuery[]> => {
  const fetch = jest.fn((_: BackendSrvRequest) => of({ data: { results: {} } }));
  setBackendSrv({ fetch } as unknown as BackendSrv);
  const now = dateTime();

  await lastValueFrom(
    newDataSource().query({
      timezone,
      range: { from: now, to: now, raw: { from: 'now', to: 'now' } },
      targets,
      scopedVars: {},
    } as unknown as DataQueryRequest<MyQuery>)
  );

  return fetch.mock.calls[0][0].data.queries;
};

const sentTimezones = async (timezone: string) =>
  (
    await sentQueries(timezone, [
      { refId: 'A', editorMode: 'builder', format: 'table' },
      { refId: 'B', editorMode: 'raw', format: 'table' },
    ])
  ).map((q) => q.timezone);

test('browser time zone resolves to the IANA zone of the browser', async () => {
  expect(await sentTimezones('browser')).toEqual(Array(2).fill(Intl.DateTimeFormat().resolvedOptions().timeZone));
});

test('empty time zone resolves to the IANA zone of the browser', async () => {
  expect(await sentTimezones('')).toEqual(Array(2).fill(Intl.DateTimeFormat().resolvedOptions().timeZone));
});

test('utc time zone is sent as UTC', async () => {
  expect(await sentTimezones('utc')).toEqual(['UTC', 'UTC']);
});

test('explicit IANA time zone is passed through', async () => {
  expect(await sentTimezones('Europe/Berlin')).toEqual(['Europe/Berlin', 'Europe/Berlin']);
});

test.each([
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
])('template variables are replaced in %s', async (key) => {
  const [sent] = await sentQueries('utc', [
    { refId: 'A', editorMode: 'builder', format: 'table', [key]: 'x $var y' } as MyQuery,
  ]);

  expect(sent[key as keyof MyQuery]).toBe('x resolved y');
});

test.each([
  ['builder query without a collection', { editorMode: 'builder' }, false],
  ['builder query with a collection', { editorMode: 'builder', collection: 'products' }, true],
  ['raw query with an empty body', { editorMode: 'raw', rawBody: '' }, false],
  ['raw query without a body', { editorMode: 'raw' }, false],
  ['raw query with a body', { editorMode: 'raw', rawBody: '{"searches":[]}' }, true],
] as const)('filterQuery: %s', (_, query, runs) => {
  expect(newDataSource().filterQuery({ refId: 'A', format: 'table', ...query })).toBe(runs);
});

test('template variables in rawBody use the json format', async () => {
  const [sent] = await sentQueries('utc', [
    { refId: 'A', editorMode: 'raw', format: 'table', rawBody: '{"filter_by":$var}' },
  ]);

  expect(sent.rawBody).toBe('{"filter_by":["a","b"]}');
});

test('filterQuery skips a raw query whose body is only whitespace', () => {
  expect(newDataSource().filterQuery({ refId: 'A', format: 'table', editorMode: 'raw', rawBody: ' \n ' })).toBe(false);
});

test('a new query starts in builder mode matching everything as a table', () => {
  expect(newDataSource().getDefaultQuery(CoreApp.PanelEditor)).toEqual({
    editorMode: 'builder',
    q: '*',
    format: 'table',
  });
});

test('a new query starts with the default collection of the data source', () => {
  expect(newDataSource({ defaultCollection: 'products' }).getDefaultQuery(CoreApp.PanelEditor)).toEqual({
    editorMode: 'builder',
    q: '*',
    format: 'table',
    collection: 'products',
  });
});

test('filterQuery skips a builder query whose collection is only whitespace', () => {
  expect(newDataSource().filterQuery({ refId: 'A', format: 'table', editorMode: 'builder', collection: '  ' })).toBe(
    false
  );
});
