# Firedoor Documentation

This directory contains the documentation for Firedoor, built with MkDocs and Material theme.

## Local Development

To work on the documentation locally:

1. **Install dependencies:**

   ```bash
   python3 -m venv mkdocs-env
   source mkdocs-env/bin/activate
   pip install mkdocs mkdocs-material
   ```

2. **Serve locally:**

   ```bash
   mkdocs serve
   ```

   The documentation will be available at <http://127.0.0.1:8000>

3. **Build static site:**

   ```bash
   mkdocs build
   ```

## Structure

- `index.md` - Homepage
- `getting-started/` - Installation and quick start guides
- `user-guide/` - Core concepts and usage patterns
- `examples/` - Real-world usage examples
- `api/` - API reference documentation
- `development/` - Contributing and development guides

## Adding Content

1. Create new markdown files in the appropriate directory
2. Update `mkdocs.yml` to include the new pages in the navigation
3. Use standard markdown with Material theme extensions

## Deployment

The documentation can be deployed to GitHub Pages using:

```bash
mkdocs gh-deploy
```

This will build the site and push it to the `gh-pages` branch.

