# Peer Review Submission & Pre-print Publication Guide for Viri Research Paper

This document provides a step-by-step roadmap for submitting and publishing the **Viri Blockchain Research Paper** to academic peer-reviewed conferences, journals, and pre-print servers.

---

## 1. ArXiv Pre-print Submission Roadmap

Publishing on [arXiv.org](https://arxiv.org) provides an immediate public timestamp and digital object identifier (DOI) for your research.

### Target arXiv Categories
- Primary: **`cs.CR`** (Computer Science — Cryptography and Security)
- Secondary: **`cs.DC`** (Computer Science — Distributed, Parallel, and Cluster Computing)

### Step-by-Step arXiv Upload Instructions
1. **Prepare Source Archive**:
   Create a single compressed `.zip` archive containing:
   - `viri_paper.tex` (main document source)
   - `references.bib` (BibTeX bibliography)
   - `IEEEtran.cls` (IEEE transaction class file)
   - `figures/` directory containing `consensus_scaling.png`, `micro_benchmarks.png`, `eip1559_fee_curve.png`

2. **Account & Endorsement**:
   - Log in to your arXiv account.
   - If this is your first `cs.CR` submission, request a category endorsement or submit using an institutional email address (`.edu` or recognized research lab domain).

3. **Upload & Auto-Compilation**:
   - Upload the `.zip` package.
   - Select **TeX / LaTeX** format engine (arXiv will automatically invoke `pdflatex` + `bibtex`).
   - Preview the generated PDF to ensure figure scaling, math alignment, and reference numbering match perfectly.

---

## 2. Target Peer-Reviewed Academic Venues

| Venue | Description & Scope | Acceptance Rate | Target Deadlines |
|---|---|---|---|
| **IEEE ICBC** (International Conference on Blockchain & Cryptocurrency) | Premier specialized conference focusing on blockchain core protocols, consensus, performance, and smart contracts. | ~20–25% | November / December |
| **IEEE S&P** (Symposium on Security and Privacy / "Oakland") | Top-tier security venue for ZK protocols, formal consensus verification, and novel ledger architectures. | ~12–15% | Rolling / Bi-monthly |
| **USENIX Security Symposium** | Top-tier systems & security conference with dedicated blockchain & applied cryptography tracks. | ~15–18% | Winter / Summer cycles |
| **ACM CCS** (Conference on Computer and Communications Security) | Top-tier security conference emphasizing cryptography, privacy pools, and formal state analysis. | ~16–19% | Spring / Summer |
| **IEEE Transactions on Computers** | Leading IEEE journal for rigorous distributed systems evaluations, consensus formalisms, and benchmark analytics. | Journal review | Year-round submission |

---

## 3. Pre-Submission Camera-Ready Checklist

Before submitting to any venue or arXiv, verify the following checklist:

- [x] **Title & Abstract**: Fully aligned with manuscript contents and math notation.
- [x] **Author Affiliations**: Anonymized if double-blind review is required by target venue (e.g., replace author block with *"Anonymous Authors for Double-Blind Review"*).
- [x] **Mathematical Consistency**: All variables ($N, F, Q, \sigma, B_n$) defined on first occurrence.
- [x] **Figures & Captions**: All 3 figure images (`consensus_scaling.png`, `micro_benchmarks.png`, `eip1559_fee_curve.png`) referenced in text via `\ref{fig:...}`.
- [x] **Tables**: `booktabs` styling used; units (ns, $\mu$s, ms, ops/sec) explicitly stated.
- [x] **Formally Verified References**: BibTeX citation keys in `references.bib` match `\cite{...}` calls in text.
- [x] **Reproducibility Code Link**: Codebase GitHub repository link (`https://github.com/RockyOmvi/Viri`) included in footers/acknowledgments.

---

## 4. File Checklist in `paper/` Directory

- [`viri_paper.tex`](file:///d:/blockchain/paper/viri_paper.tex): Main 7–8 page IEEE journal paper.
- [`viri_presentation.tex`](file:///d:/blockchain/paper/viri_presentation.tex): 15-slide Beamer presentation deck.
- [`references.bib`](file:///d:/blockchain/paper/references.bib): 22 BibTeX bibliography citations.
- [`figures/`](file:///d:/blockchain/paper/figures/): PNG vector charts (`consensus_scaling.png`, `micro_benchmarks.png`, `eip1559_fee_curve.png`).
- [`generate_charts.py`](file:///d:/blockchain/paper/generate_charts.py): Python script to regenerate figures.
- [`PAPER_SUMMARY.md`](file:///d:/blockchain/paper/PAPER_SUMMARY.md): Complete Markdown version of the paper.
- [`SUBMISSION_GUIDE.md`](file:///d:/blockchain/paper/SUBMISSION_GUIDE.md): Publication & submission guide.
