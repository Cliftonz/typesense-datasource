import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryEditor } from './QueryEditor';
import { DataSource } from '../datasource';
import { MyQuery } from '../types';

jest.mock('@grafana/ui', () => ({
  ...jest.requireActual('@grafana/ui'),
  CodeEditor: ({ value, onBlur }: { value: string; onBlur: (value: string) => void }) => (
    <textarea aria-label="Raw body" defaultValue={value} onBlur={(e) => onBlur(e.currentTarget.value)} />
  ),
}));

const datasource = (canList = false, collections: string[] = []) =>
  ({ getResource: jest.fn().mockResolvedValue({ canList, collections }) }) as unknown as DataSource;

const baseQuery: MyQuery = { refId: 'A', editorMode: 'builder', q: '*', format: 'table' };

afterEach(() => jest.restoreAllMocks());

const renderEditor = (query: Partial<MyQuery> = {}, ds = datasource(), onRunQuery = jest.fn()) => {
  const onChange = jest.fn();
  render(
    <QueryEditor datasource={ds} query={{ ...baseQuery, ...query }} onChange={onChange} onRunQuery={onRunQuery} />
  );
  return onChange;
};

test.each([
  ['Query', 'q', 'shoes'],
  ['Query by', 'queryBy', 'title,description'],
  ['Filter by', 'filterBy', 'price:>10'],
  ['Sort by', 'sortBy', 'price:desc'],
  ['Facet by', 'facetBy', 'brand'],
  ['Group by', 'groupBy', 'category'],
])('builder field %s writes %s', async (label, key, value) => {
  const onChange = renderEditor();

  fireEvent.change(await screen.findByRole('textbox', { name: label }), { target: { value } });

  expect(onChange.mock.calls[0][0][key]).toBe(value);
});

test('per page writes a number to perPage', async () => {
  const onChange = renderEditor();

  fireEvent.change(await screen.findByRole('spinbutton', { name: 'Per page' }), { target: { value: '50' } });

  expect(onChange.mock.calls[0][0].perPage).toBe(50);
});

test('collection is a text input when the backend cannot list collections', async () => {
  const ds = datasource(false);
  const onChange = renderEditor({}, ds);

  fireEvent.change(await screen.findByRole('textbox', { name: 'Collection' }), { target: { value: 'products' } });

  expect(ds.getResource).toHaveBeenCalledWith('collections');
  expect(screen.queryByRole('combobox', { name: 'Collection' })).not.toBeInTheDocument();
  expect(onChange.mock.calls[0][0].collection).toBe('products');
});

test('collection is a combobox of listed names that accepts a custom value when the backend can list', async () => {
  jest.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(300);
  jest.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(300);
  const onChange = renderEditor({}, datasource(true, ['products', 'orders']));

  const collection = await screen.findByRole('combobox', { name: 'Collection' });
  fireEvent.click(collection);
  expect(await screen.findByRole('option', { name: 'orders' })).toBeInTheDocument();

  fireEvent.change(collection, { target: { value: 'archived' } });
  fireEvent.keyDown(collection, { key: 'Enter' });

  expect(onChange.mock.calls[0][0].collection).toBe('archived');
});

test('editor mode toggle switches editorMode to raw', async () => {
  const onChange = renderEditor();

  fireEvent.click(await screen.findByRole('radio', { name: 'Raw' }));

  expect(onChange.mock.calls[0][0].editorMode).toBe('raw');
});

test('raw mode shows the raw body editor instead of the builder fields', async () => {
  renderEditor({ editorMode: 'raw', rawBody: '{"searches":[]}' });

  expect(await screen.findByRole('textbox', { name: 'Raw body' })).toHaveValue('{"searches":[]}');
  expect(screen.queryByRole('textbox', { name: 'Query by' })).not.toBeInTheDocument();
});

test('raw body editor writes rawBody on blur', async () => {
  const onChange = renderEditor({ editorMode: 'raw' });

  const rawBody = await screen.findByRole('textbox', { name: 'Raw body' });
  fireEvent.change(rawBody, { target: { value: '{"searches":[{"collection":"products"}]}' } });
  fireEvent.blur(rawBody);

  expect(onChange.mock.calls[0][0].rawBody).toBe('{"searches":[{"collection":"products"}]}');
});

test('format toggle writes timeseries to format', async () => {
  const onChange = renderEditor();

  fireEvent.click(await screen.findByRole('radio', { name: 'Time series' }));

  expect(onChange.mock.calls[0][0].format).toBe('timeseries');
});

test('time series controls are hidden for table format', async () => {
  renderEditor({ format: 'table' });

  expect(await screen.findByRole('radio', { name: 'Table' })).toBeInTheDocument();
  expect(screen.queryByRole('textbox', { name: 'Value field' })).not.toBeInTheDocument();
  expect(screen.queryByRole('radio', { name: 'Hits' })).not.toBeInTheDocument();
});

test.each([
  ['Time field', 'timeField', 'created_at'],
  ['Value field', 'valueField', 'price'],
  ['Min interval', 'minInterval', '1m'],
])('time series field %s writes %s', async (label, key, value) => {
  const onChange = renderEditor({ format: 'timeseries' });

  fireEvent.change(await screen.findByRole('textbox', { name: label }), { target: { value } });

  expect(onChange.mock.calls[0][0][key]).toBe(value);
});

test('bucket mode toggle writes bucketMode', async () => {
  const onChange = renderEditor({ format: 'timeseries' });

  fireEvent.click(await screen.findByRole('radio', { name: 'Range facet' }));

  expect(onChange.mock.calls[0][0].bucketMode).toBe('rangeFacet');
});

test('stats switch writes stats', async () => {
  const onChange = renderEditor({ format: 'timeseries', bucketMode: 'hits' });

  fireEvent.click(await screen.findByRole('switch', { name: /Stats/ }));

  expect(onChange.mock.calls[0][0].stats).toBe(true);
});

test('stats switch is hidden for groupBy bucket mode', async () => {
  renderEditor({ format: 'timeseries', bucketMode: 'groupBy' });

  expect(await screen.findByRole('textbox', { name: 'Time field' })).toBeInTheDocument();
  expect(screen.queryByRole('switch', { name: /Stats/ })).not.toBeInTheDocument();
});

test('max pages writes a number to maxPages for hits bucket mode', async () => {
  const onChange = renderEditor({ format: 'timeseries', bucketMode: 'hits' });

  fireEvent.change(await screen.findByRole('spinbutton', { name: 'Max pages' }), { target: { value: '20' } });

  expect(onChange.mock.calls[0][0].maxPages).toBe(20);
});

test('max pages is hidden for rangeFacet bucket mode', async () => {
  renderEditor({ format: 'timeseries', bucketMode: 'rangeFacet' });

  expect(await screen.findByRole('textbox', { name: 'Time field' })).toBeInTheDocument();
  expect(screen.queryByRole('spinbutton', { name: 'Max pages' })).not.toBeInTheDocument();
});

test('max pages writes a number to maxPages for groupBy bucket mode', async () => {
  const onChange = renderEditor({ format: 'timeseries', bucketMode: 'groupBy' });

  fireEvent.change(await screen.findByRole('spinbutton', { name: 'Max pages' }), { target: { value: '20' } });

  expect(onChange.mock.calls[0][0].maxPages).toBe(20);
});

test('raw mode hides format and time series controls', async () => {
  renderEditor({ editorMode: 'raw', format: 'timeseries', bucketMode: 'hits' });

  expect(await screen.findByRole('textbox', { name: 'Raw body' })).toBeInTheDocument();
  expect(screen.queryByRole('radio', { name: 'Time series' })).not.toBeInTheDocument();
  expect(screen.queryByRole('radio', { name: 'Hits' })).not.toBeInTheDocument();
  expect(screen.queryByRole('textbox', { name: 'Time field' })).not.toBeInTheDocument();
});

test('value field is hidden for groupBy bucket mode', async () => {
  renderEditor({ format: 'timeseries', bucketMode: 'groupBy' });

  expect(await screen.findByRole('textbox', { name: 'Time field' })).toBeInTheDocument();
  expect(screen.queryByRole('textbox', { name: 'Value field' })).not.toBeInTheDocument();
});

test('time field is shown for table format and writes timeField', async () => {
  const onChange = renderEditor({ format: 'table' });

  fireEvent.change(await screen.findByRole('textbox', { name: 'Time field' }), { target: { value: 'created_at' } });

  expect(onChange.mock.calls[0][0].timeField).toBe('created_at');
});

test.each([
  ['Per page', 'perPage'],
  ['Max pages', 'maxPages'],
])('clearing %s removes %s', async (label, key) => {
  const onChange = renderEditor({ format: 'timeseries', bucketMode: 'hits', perPage: 50, maxPages: 20 });

  fireEvent.change(await screen.findByRole('spinbutton', { name: label }), { target: { value: '' } });

  expect(onChange.mock.calls[0][0]).not.toHaveProperty(key);
});

test('collection is a text input when listing collections fails', async () => {
  const getResource = jest.fn().mockRejectedValue(new Error('Not found'));
  renderEditor({}, { getResource } as unknown as DataSource);

  await waitFor(() => expect(getResource).toHaveBeenCalled());
  expect(await screen.findByRole('textbox', { name: 'Collection' })).toBeInTheDocument();
});

test('switching to time series defaults bucketMode to hits with stats off and maxPages unset', async () => {
  const onChange = renderEditor();

  fireEvent.click(await screen.findByRole('radio', { name: 'Time series' }));

  const changed = onChange.mock.calls[0][0];
  expect(changed.bucketMode).toBe('hits');
  expect(changed.stats).toBeFalsy();
  expect(changed).not.toHaveProperty('maxPages');
});

test('switching to time series keeps a chosen bucketMode', async () => {
  const onChange = renderEditor({ bucketMode: 'rangeFacet' });

  fireEvent.click(await screen.findByRole('radio', { name: 'Time series' }));

  expect(onChange.mock.calls[0][0].bucketMode).toBe('rangeFacet');
});

test.each([
  ['textbox', 'Collection'],
  ['textbox', 'Query by'],
  ['textbox', 'Time field'],
  ['textbox', 'Min interval'],
  ['spinbutton', 'Per page'],
  ['spinbutton', 'Max pages'],
  ['textbox', 'Raw body'],
])('blurring %s %s runs the query', async (role, name) => {
  const onRunQuery = jest.fn();
  const editorMode = name === 'Raw body' ? 'raw' : 'builder';
  renderEditor({ editorMode, format: 'timeseries', bucketMode: 'hits' }, datasource(), onRunQuery);

  const field = await screen.findByRole(role, { name });
  expect(onRunQuery).not.toHaveBeenCalled();
  fireEvent.blur(field);

  expect(onRunQuery).toHaveBeenCalled();
});

test.each([
  ['radio', 'Raw'],
  ['radio', 'Time series'],
  ['radio', 'Range facet'],
  ['switch', /Stats/],
])('changing %s %s runs the query', async (role, name) => {
  const onRunQuery = jest.fn();
  renderEditor(
    { format: name === 'Time series' ? 'table' : 'timeseries', bucketMode: 'hits' },
    datasource(),
    onRunQuery
  );

  fireEvent.click(await screen.findByRole(role, { name }));

  expect(onRunQuery).toHaveBeenCalled();
});

test('choosing a collection from the combobox runs the query', async () => {
  jest.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(300);
  jest.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(300);
  const onRunQuery = jest.fn();
  renderEditor({}, datasource(true, ['products', 'orders']), onRunQuery);

  fireEvent.click(await screen.findByRole('combobox', { name: 'Collection' }));
  fireEvent.click(await screen.findByRole('option', { name: 'orders' }));

  expect(onRunQuery).toHaveBeenCalled();
});

test('switching to table leaves an unset bucketMode unset', async () => {
  const onChange = renderEditor({ format: 'timeseries' });

  fireEvent.click(await screen.findByRole('radio', { name: 'Table' }));

  expect(onChange.mock.calls[0][0]).not.toHaveProperty('bucketMode');
});

test('per page input is limited to whole numbers from 1 to 250', async () => {
  renderEditor();

  const field = await screen.findByRole('spinbutton', { name: 'Per page' });

  expect(field).toHaveAttribute('min', '1');
  expect(field).toHaveAttribute('max', '250');
  expect(field).toHaveAttribute('step', '1');
});

test('max pages input is limited to whole numbers from 1 up', async () => {
  renderEditor({ format: 'timeseries', bucketMode: 'hits' });

  const field = await screen.findByRole('spinbutton', { name: 'Max pages' });

  expect(field).toHaveAttribute('min', '1');
  expect(field).not.toHaveAttribute('max');
  expect(field).toHaveAttribute('step', '1');
});

test.each([
  ['Per page', '0'],
  ['Per page', '251'],
  ['Per page', '2.5'],
  ['Max pages', '0'],
  ['Max pages', '1.5'],
])('%s does not write %s', async (label, value) => {
  const onChange = renderEditor({ format: 'timeseries', bucketMode: 'hits' });

  fireEvent.change(await screen.findByRole('spinbutton', { name: label }), { target: { value } });

  expect(onChange).not.toHaveBeenCalled();
});

test.each([
  ['Per page', 'perPage', '1', 1],
  ['Per page', 'perPage', '250', 250],
  ['Max pages', 'maxPages', '1', 1],
  ['Max pages', 'maxPages', '1000', 1000],
])('%s writes boundary value %s', async (label, key, value, written) => {
  const onChange = renderEditor({ format: 'timeseries', bucketMode: 'hits' });

  fireEvent.change(await screen.findByRole('spinbutton', { name: label }), { target: { value } });

  expect(onChange.mock.calls[0][0][key]).toBe(written);
});
