import { test, expect } from '@grafana/plugin-e2e';

test('raw mode keeps the typed multi_search body', async ({
  panelEditPage,
  readProvisionedDataSource,
  selectors,
  page,
}) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  const row = panelEditPage.getQueryEditorRow('A');

  await row.getByRole('radio', { name: 'Raw' }).click({ force: true });
  await expect(row.getByRole('radio', { name: 'Raw' })).toBeChecked();
  const editor = panelEditPage.getByGrafanaSelector(selectors.components.CodeEditor.container, { root: row });
  await editor.locator('.view-lines').click();
  await page.keyboard.insertText('{"searches":[{"collection":"products","q":"*"}]}');
  await expect(editor).toContainText('"collection":"products"');
  await row.getByRole('radio', { name: 'Builder' }).click({ force: true });
  await expect(row.getByRole('radio', { name: 'Builder' })).toBeChecked();
  await row.getByRole('radio', { name: 'Raw' }).click({ force: true });

  await expect(editor).toContainText('"collection":"products"');
});
