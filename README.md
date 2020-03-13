# bundlesplit

[![License](https://img.shields.io/github/license/seankhliao/bundlesplit.svg?style=flat-square)](LICENSE)
![Version](https://img.shields.io/github/v/tag/seankhliao/bundlesplit?sort=semver&style=flat-square)
[![Godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](https://godoc.org/github.com/seankhliao/bundlesplit)

split k8s bundles into a kustomize tree structure

## Install

```bash
GO111MODULE=on go get github.com/seankhliao/bundlesplit
```

## Usage

```bash
bundlesplit appname FILE
bundlesplit appname URL
```

results in

```txt
root <- you are here
  |- kustomization.yaml
  `- namespace1
      |- kustomization.yaml
      ` appname1
          |- kustomization.yaml
          |- Deplyment.yaml
          |- Service.yaml
          `- ...
```
