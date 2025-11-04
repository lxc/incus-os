import datetime
import os
import subprocess
import yaml

from git import Repo

# Download and link swagger-ui files
if not os.path.isdir('.sphinx/deps/swagger-ui'):
    Repo.clone_from('https://github.com/swagger-api/swagger-ui', '.sphinx/deps/swagger-ui', depth=1)

os.makedirs('.sphinx/_static/swagger-ui/', exist_ok=True)

if not os.path.islink('.sphinx/_static/swagger-ui/swagger-ui-bundle.js'):
    os.symlink('../../deps/swagger-ui/dist/swagger-ui-bundle.js', '.sphinx/_static/swagger-ui/swagger-ui-bundle.js')
if not os.path.islink('.sphinx/_static/swagger-ui/swagger-ui-standalone-preset.js'):
    os.symlink('../../deps/swagger-ui/dist/swagger-ui-standalone-preset.js', '.sphinx/_static/swagger-ui/swagger-ui-standalone-preset.js')
if not os.path.islink('.sphinx/_static/swagger-ui/swagger-ui.css'):
    os.symlink('../../deps/swagger-ui/dist/swagger-ui.css', '.sphinx/_static/swagger-ui/swagger-ui.css')

# Project config.
project = "IncusOS"
author = "IncusOS contributors"
copyright = "2024-%s %s" % (datetime.date.today().year, author)

try:
    version = subprocess.check_output(["git", "describe", "--tags", "--abbrev=0"])
    version = str(version.strip())
except:
    version = "dev"

# Extensions.
extensions = [
    "config-options",
    "custom-rst-roles",
    "myst_parser",
    "notfound.extension",
    "related-links",
    "sphinxcontrib.jquery",
    "sphinx_copybutton",
    "sphinx_design",
    "sphinx.ext.intersphinx",
    "sphinxext.opengraph",
    "sphinx_remove_toctrees",
    "sphinx_reredirects",
    "sphinx_tabs.tabs",
    "terminal-output",
    "youtube-links",
]

myst_enable_extensions = ["deflist", "linkify", "substitution"]

myst_linkify_fuzzy_links = False
myst_heading_anchors = 7

if os.path.exists("./substitutions.yaml"):
    with open("./substitutions.yaml", "r") as fd:
        myst_substitutions = yaml.safe_load(fd.read())
if os.path.exists("./related_topics.yaml"):
    with open("./related_topics.yaml", "r") as fd:
        myst_substitutions.update(yaml.safe_load(fd.read()))

intersphinx_mapping = {"incus": ("https://linuxcontainers.org/incus/docs/main/", None)}

myst_url_schemes = {
    "http": None,
    "https": None,
}

# Setup theme.
html_theme = "furo"
html_show_sphinx = False
html_last_updated_fmt = ""
html_favicon = ".sphinx/_static/favicon.ico"
html_static_path = [".sphinx/_static"]
html_css_files = ["custom.css", "furo_colors.css"]

html_theme_options = {
    "sidebar_hide_name": True,
}

html_context = {
    "github_url": "https://github.com/lxc/incus-os",
    "github_version": "main",
    "github_folder": "/doc/",
    "github_filetype": "md",
    "discourse_prefix": {"lxc": "https://discuss.linuxcontainers.org/t/"},
}

source_suffix = ".md"

# List of patterns, relative to source directory, that match files and
# directories to ignore when looking for source files.
# This pattern also affects html_static_path and html_extra_path.
exclude_patterns = ["html", "README.md", ".sphinx", "config_options_cheat_sheet.md"]

# Open Graph configuration

ogp_site_url = "https://linuxcontainers.org/incus-os/docs/main/"
ogp_site_name = "IncusOS documentation"
ogp_image = "https://linuxcontainers.org/static/img/containers.png"

# Links to ignore when checking links

linkcheck_ignore = ["https://web.libera.chat/#lxc", r"https://uefi.org/.*"]

linkcheck_anchors_ignore_for_url = [r"https://github\.com/.*"]
