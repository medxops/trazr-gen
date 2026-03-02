# Product Requirements Document: AI-Powered Synthetic Healthcare Data Generator

**Version:** 1.8  
**Date:** May 23, 2025  
**Author:** Gemini AI  
**Status:** Draft

---

## 1. Introduction

This document outlines the product requirements for a Go-based command-line interface (CLI) program designed to generate realistic synthetic data. The program focuses on creating data based on predefined, industry-standard categories such as:

- **Protected Health Information (PHI_HIPAA)**
- **GDPR Personal Data (GDPR_PersonalData)**
- **Payment Card Industry Data (PCI_CardholderData)**
- **General Personally Identifiable Information (General_PII - guided by NIST principles)**
- **Technical Secrets (Secrets)**
- **User_Custom_Data** (user-provided prompts)

A key advanced feature is the optional generation of suggested regular expressions (regex) for detecting the synthetic data elements, intended to aid in the development and refinement of data detection tools.

The program will leverage the OpenAI GPT-4 series of models (e.g., gpt-4.1, gpt-4o) through direct API calls, using carefully crafted system prompts (which users can override for standard categories) and OpenAI's structured output capabilities. Key features include a validation and correction loop with the LLM to improve output quality. Generated data for each selected category, including custom ones and optional regex patterns, will be output into separate CSV files, intended for consumption by external processes. The primary goal is to provide a flexible, standard-aware, and customizable tool for testing, development, and demonstration.

---

## 2. Goals

- Develop a CLI tool to generate configurable synthetic data.
- Utilize OpenAI's GPT-4 series models for generating realistic and varied data points.
- Allow users to select one or more standard data categories (e.g., PHI_HIPAA, GDPR_PersonalData, etc.) for generation.
- Support a User_Custom_Data category where data generation is driven entirely by a user-supplied prompt.
- Enable user-override of system prompts for selected standard categories or specific field types within them.
- Implement an advanced option to request the LLM to provide a suggested regex pattern for each generated synthetic data element, where applicable.
- Implement a validation and correction loop with the LLM to ensure data conforms to expected JSON structures.
- Support CSV output format, with each selected category generating its own file, including regex data if enabled.
- Enable data generation for multiple languages and locales.
- Allow customization of output field headers.
- Output a status report summarizing the data generation process.
- Ensure the tool is easy to configure and use.

---

## 3. Target Users

- **Software Developers:** Building and testing applications that process sensitive data under various compliance regimes (HIPAA, GDPR, PCI DSS, etc.).
- **QA Engineers/Testers:** Creating test datasets for validating software functionality and data handling.
- **Data Privacy Officers/Compliance Teams:** Generating sample data for training, awareness, or testing anonymization/pseudonymization tools.
- **Data Scientists/Analysts:** Developing and testing models or analytics without using real sensitive data.
- **Security Professionals:** Simulating data for security testing, penetration testing, and training.
- **Sales/Demo Teams:** Demonstrating product capabilities with realistic-looking but entirely synthetic data.

---

## 4. Requirements

### 4.1. Functional Requirements

#### 4.1.1. AI-Powered Data Generation (OpenAI)

- **FR1.1:** The system must directly interface with the OpenAI API to generate synthetic data.
- **FR1.2:** The specific OpenAI model shall be configurable. Default API parameters will be preset for optimal performance; the API key is mandatory.
- **FR1.3:**
  - For standard categories, the program uses system prompts tailored to the category and its field types.
  - For the User_Custom_Data category, the user-provided prompt is used directly.
  - All prompts will explicitly request JSON output.
  - If the GenerateDetectionRegex option (see FR7.2) is enabled, prompts will be augmented to instruct the LLM to also provide a suggested regex pattern for detecting each generated data element, where applicable.
- **FR1.4:** Users can override base system prompts for standard categories or specific field types within them. For User_Custom_Data, the user provides the entire generation prompt. If GenerateDetectionRegex is enabled, the system will append a general instruction to the user's custom prompt to also attempt regex generation for the custom fields.
- **FR1.5:** Leverage OpenAI's "Structured Outputs" or "JSON mode."
- **FR1.6:** Standard and Custom Data Categories:

  1. **PHI_HIPAA**
      - *Description:* Encompasses data elements defined as PHI under HIPAA.
      - *Representative Field Types:* PatientFullName, DateOfBirth, StreetAddress, City, State, ZipCode, County, TelephoneNumber, FaxNumber, EmailAddress, SocialSecurityNumber_Mock, MedicalRecordNumber_Mock, HealthPlanBeneficiaryNumber_Mock, AccountNumber_Financial_Mock, CertificateLicenseNumber_Mock, VehicleLicensePlateNumber_Mock, DeviceIdentifier_Mock, WebURL_PatientPortal_Mock, IPAddress_Mock, BiometricIdentifierHash_Mock (e.g., for fingerprint, voiceprint), FullFacePhotoPlaceholderText, AdmissionDate, DischargeDate, DateOfDeath, DiagnosisText (ICD-10 guided), ProcedureText (CPT guided), MedicationText (RxNorm guided), LabTestName (LOINC guided), LabTestResultText_Mock, AttendingPhysicianName_Mock, HospitalName_Mock, OtherUniqueIdentifyingCode_Mock.

  2. **GDPR_PersonalData**
      - *Description:* Covers data defined as "personal data" under GDPR Art. 4(1), including sensitive categories from Art. 9.
      - *Representative Field Types:* SubjectFullName, HomeAddress_GDPR, EmailAddress_GDPR, IdentificationCardNumber_Mock_GDPR, LocationData_Coordinates_Mock, IPAddress_Mock_GDPR, CookieID_Mock, AdvertisingIdentifier_Mock, HealthDataText_GDPR (mock, within GDPR context), GeneticDataHash_GDPR_Mock, BiometricDataHash_GDPR_Mock (for unique ID), RacialEthnicOriginText_Mock (generated with ethical consideration if explicitly prompted), PoliticalOpinionText_Mock (generated with ethical consideration if explicitly prompted), ReligiousPhilosophicalBeliefText_Mock (generated with ethical consideration if explicitly prompted), TradeUnionMembershipText_Mock (generated with ethical consideration if explicitly prompted), SexLifeSexualOrientationText_Mock (generated with ethical consideration if explicitly prompted).

  3. **PCI_CardholderData**
      - *Description:* Covers key elements of cardholder data as defined by PCI DSS. Generation will focus on mock data that mimics the format and structure for testing systems, not for creating valid or usable card details.
      - *Representative Field Types:* PrimaryAccountNumber_PAN_Mock (Luhn-valid format if possible, but clearly test data), CardholderName_PCI, ExpirationDate_PCI_Mock (MM/YY or MM/YYYY), ServiceCode_PCI_Mock, CVV_CVC_CID_PlaceholderText (e.g., "CVV_MOCK_PRESENT" or "NOT_STORED_MOCK"), TrackDataEquivalent_PlaceholderText (e.g., "TRACK_DATA_NOT_APPLICABLE_MOCK"), PINBlock_PlaceholderText (e.g., "PIN_PROCESSING_NOT_SIMULATED_MOCK").

  4. **General_PII**
      - *Description:* Broader PII not strictly tied to one regulation but commonly protected, guided by principles from NIST SP 800-122 (including direct and linkable identifiers).
      - *Representative Field Types:* FullName_PII, MailingAddress_PII, EmailAddress_PII, PhoneNumber_PII, DateOfBirth_PII, AccountUsername_Mock, GovernmentIssuedID_Generic_Mock (e.g., Driver's License format), FinancialAccountNumber_Generic_Mock (non-PCI bank account), LoginPassword_Mock, EmploymentInformation_Mock (e.g., Job Title, Employer Name), EducationalInformation_Mock (e.g., Highest Degree, Institution Name).

  5. **Secrets**
      - *Description:* For generating mock technical secrets used in IT systems.
      - *Representative Field Types:* APIKeyString_Mock (e.g., OpenAI, AWS format), PasswordString_Complex_Mock, AccessTokenString_JWT_Mock, EncryptionKeyString_AES_Mock, DBConnectionString_Mock (with placeholders), PrivateKeyString_RSA_Mock, SSHKeyString_Private_Mock.

  6. **User_Custom_Data (User-Defined via Custom Prompt)**
      - *Description:* Allows users to generate data based entirely on a specific prompt they provide in the configuration. The structure of the output will depend on what the user's prompt instructs the LLM to produce. This category will only be generated if a customPrompt is provided for it in the configuration.
      - *Representative Field Types:* Dynamically determined by the JSON keys returned by the LLM in response to the user-supplied customPrompt.

- **FR1.7:**
  - If GenerateDetectionRegex is false: Internal representation for a category record is a JSON object where keys are field names and values are the synthetic data.
  - If GenerateDetectionRegex is true: Internal representation for a category record is a JSON object where each field name maps to a nested JSON object: `{ "value": "synthetic_data_here", "detectionRegex": "suggested_regex_here_or_null" }`. This structure applies to fields in standard categories and fields generated by User_Custom_Data. The LLM may return null or an empty string for detectionRegex if not applicable or determinable.

- **FR1.8:** Fail fast on missing/invalid API key.

#### 4.1.2. Data Validation and Correction Loop

- **FR2.1:** Parse LLM response as JSON according to the expected structure (simple key-value or nested value/regex based on GenerateDetectionRegex option).
- **FR2.2 (Phase 1):** Basic JSON validation (well-formed and matches the expected top-level structure).
- **FR2.3:** If parsing fails, initiate correction attempt.
- **FR2.4:** Correction loop: resend original prompt + erroneous response + error message.
- **FR2.5:** Configurable max retries (default: 2).
- **FR2.6:** If LLM fails after retries, log error, skip record (behavior configurable).

#### 4.1.3. Language and Locale Support

- **FR3.1:** Users can specify target languages/locales.
- **FR3.2:** Generated data values conform to locale conventions.
- **FR3.3:** Output field headers:
  - If GenerateDetectionRegex is false: Default to "Representative Field Type" names or keys from custom prompt's JSON.
  - If GenerateDetectionRegex is true: For each original field FieldName, two columns will be generated: FieldName_Value and FieldName_Regex.
  - Users can specify a different language for headers.

#### 4.1.4. Output Formatting and Reporting

- **FR4.1 (Phase 1):** Output format is CSV. Each selected category generates its own CSV file.
- **FR4.2:** Each row is a synthetic record.
  - If GenerateDetectionRegex is false: Columns correspond to the field names.
  - If GenerateDetectionRegex is true: For each logical piece of data, there will be a _Value column and a _Regex column (e.g., PatientFullName_Value, PatientFullName_Regex).
- **FR4.3:** Handle special characters in CSV.
- **FR4.4:** Console status report:
  - For each category: requested, successful, failed.
  - Indicate if User_Custom_Data was skipped due to missing prompt.
  - Indicate if regex generation was active.
  - Significant errors.

#### 4.1.5. Data Element Consistency

- **FR5.1 (Phase 1):** Rely on LLM common sense.

#### 4.1.6. Data Injection (Responsibility Clarification)

- **FR6.1:** Program generates CSV files.
- **FR6.2:** Injection handled externally.

#### 4.1.7. Configuration

- **FR7.1:** Configurable via file/CLI.
- **FR7.2:** Configuration options:
  - OpenAI API Key (mandatory).
  - OpenAI Model ID.
  - GenerateDetectionRegex: Boolean, default false. If true, instructs the LLM to attempt to provide a regex for each generated data element.
  - CategoriesToGenerate: A list specifying which categories to generate, with count and specific overrides. Example:

```yaml
# Configuration file format: YAML
CategoriesToGenerate:
  - categoryName: "PHI_HIPAA"
    count: 100
    # ... other overrides
  - categoryName: "User_Custom_Data"
    outputAlias: "MyCompanyProfiles" # Results in output_mycompanyprofiles.csv
    count: 10 # Number of times to run this custom prompt
    # 'customPrompt' is MANDATORY for 'User_Custom_Data' to be generated.
    customPrompt: "Generate a synthetic company profile including company name, founding year (between 1980-2020), a brief mission statement (1-2 sentences), number of employees (10-1000), and a mock CEO name. Return as a JSON object."
# ... etc.
```

  - Target languages and locales.
  - Output header language.
  - Output directory path.
  - Max_LLM_Retries (default: 2).
  - Behavior on persistent LLM failure.

### 4.2. Non-Functional Requirements

(No changes from Version 1.7)

---

## 5. Phase 1 Scope

- Generation for predefined standard categories and User_Custom_Data.
- Optional regex generation alongside synthetic data.
- User-configurable system prompt overrides; user-supplied prompts for User_Custom_Data.
- Basic JSON parsing validation & LLM correction loop.
- Support for en-CA, fr-CA, en-US locales.
- Separate CSV file output per generated category, with conditional regex columns.
- Generation status reporting.
- Rely on LLM for intra-record data consistency.
- Fail fast on missing/invalid API key.

---

## 6. Future Considerations (Post Phase 1)

- Advanced Data Relationships.
- Hybrid Category Definition.
- Deeper Schema Validation (including validation of LLM-generated regex patterns for basic correctness if possible).
- More Output Formats.
- GUI Interface.
- Advanced Prompt Management & Versioning.
- Configurable OpenAI API parameters.
- User-provided regex patterns: Allow users to provide known good regex for certain data types, which the system could then use for validation if the LLM also generates one, or use as part of the prompt to guide the LLM.

---

## 7. Resolved Questions (Summary of Decisions)

- **OQ1 (Correction Loop Strategy):** Simple error resend (original prompt + error message). Default retries: 2.
- **OQ2 (OpenAI API Parameters):** Phase 1: critical parameters like temperature/top_p will be preset for optimal performance; API key is mandatory. Advanced parameter configuration is a future consideration.
- **OQ3 (JSON Validation Complexity):** Phase 1: basic (well-formed JSON). Deeper data validation is for a later phase.
- **OQ4 (API Key Behavior):** Fail fast if the API key is missing or invalid.
- **OQ5 (Healthcare Terminologies):** AI generation will be guided by major, commonly recognized terminologies (e.g., ICD-10, CPT, LOINC, RxNorm) for relevant healthcare text elements. User prompt overrides can refine.
- **OQ6 (Contradictory Data Elements):** Phase 1: rely on LLM common sense. Pre-validation of profile definitions or complex post-generation validation for contradictions is out of scope.

