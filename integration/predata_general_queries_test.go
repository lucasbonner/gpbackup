package integration

import (
	"github.com/greenplum-db/gpbackup/backup"
	"github.com/greenplum-db/gpbackup/testutils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("backup integration tests", func() {
	BeforeEach(func() {
		testutils.SetupTestLogger()
	})
	Describe("GetAllUserSchemas", func() {
		It("returns user schema information", func() {
			testutils.AssertQueryRuns(connection, "CREATE SCHEMA bar")
			defer testutils.AssertQueryRuns(connection, "DROP SCHEMA bar")
			schemas := backup.GetAllUserSchemas(connection)

			schemaBar := backup.Schema{0, "bar"}
			schemaPublic := backup.Schema{2200, "public"}

			Expect(len(schemas)).To(Equal(2))
			testutils.ExpectStructsToMatchExcluding(&schemaBar, &schemas[0], "Oid")
			testutils.ExpectStructsToMatchExcluding(&schemaPublic, &schemas[1], "Owner")
		})
	})
	Describe("GetProceduralLanguages", func() {
		It("returns a slice of procedural languages", func() {
			testutils.AssertQueryRuns(connection, "CREATE LANGUAGE plperl")
			defer testutils.AssertQueryRuns(connection, "DROP LANGUAGE plperl")

			pgsqlHandlerOid := backup.OidFromObjectName(connection, "pg_catalog", "plpgsql_call_handler", backup.FunctionParams)
			pgsqlInlineOid := backup.OidFromObjectName(connection, "pg_catalog", "plpgsql_inline_handler", backup.FunctionParams)
			pgsqlValidatorOid := backup.OidFromObjectName(connection, "pg_catalog", "plpgsql_validator", backup.FunctionParams)

			perlHandlerOid := backup.OidFromObjectName(connection, "pg_catalog", "plperl_call_handler", backup.FunctionParams)
			perlInlineOid := backup.OidFromObjectName(connection, "pg_catalog", "plperl_inline_handler", backup.FunctionParams)
			perlValidatorOid := backup.OidFromObjectName(connection, "pg_catalog", "plperl_validator", backup.FunctionParams)

			expectedPlpgsqlInfo := backup.QueryProceduralLanguage{0, "plpgsql", "testrole", true, true, pgsqlHandlerOid, pgsqlInlineOid, pgsqlValidatorOid}
			expectedPlperlInfo := backup.QueryProceduralLanguage{1, "plperl", "testrole", true, true, perlHandlerOid, perlInlineOid, perlValidatorOid}

			resultProcLangs := backup.GetProceduralLanguages(connection)

			Expect(len(resultProcLangs)).To(Equal(2))
			testutils.ExpectStructsToMatchExcluding(&expectedPlpgsqlInfo, &resultProcLangs[0], "Oid", "Owner")
			testutils.ExpectStructsToMatchExcluding(&expectedPlperlInfo, &resultProcLangs[1], "Oid", "Owner")
		})
	})
	Describe("GetOperators", func() {
		It("returns a slice of operators", func() {
			testutils.AssertQueryRuns(connection, "CREATE OPERATOR ## (LEFTARG = bigint, PROCEDURE = numeric_fac)")
			defer testutils.AssertQueryRuns(connection, "DROP OPERATOR ## (bigint, NONE)")

			expectedOperator := backup.QueryOperator{0, "public", "##", "numeric_fac", "bigint", "-", "0", "0", "-", "-", false, false}

			results := backup.GetOperators(connection)

			Expect(len(results)).To(Equal(1))
			testutils.ExpectStructsToMatchExcluding(&expectedOperator, &results[0], "Oid")
		})
		It("returns a slice of operators with a full-featured operator", func() {
			testutils.AssertQueryRuns(connection, "CREATE SCHEMA testschema")
			defer testutils.AssertQueryRuns(connection, "DROP SCHEMA testschema")

			testutils.AssertQueryRuns(connection, "CREATE FUNCTION testschema.\"testFunc\"(path,path) RETURNS path AS 'SELECT $1' LANGUAGE SQL IMMUTABLE")
			defer testutils.AssertQueryRuns(connection, "DROP FUNCTION testschema.\"testFunc\"(path,path)")

			testutils.AssertQueryRuns(connection, `
			CREATE OPERATOR testschema.## (
				LEFTARG = path,
				RIGHTARG = path,
				PROCEDURE = testschema."testFunc",
				COMMUTATOR = OPERATOR(testschema.##),
				NEGATOR = OPERATOR(public.###),
				RESTRICT = eqsel,
				JOIN = eqjoinsel,
				HASHES,
				MERGES
			)`)
			defer testutils.AssertQueryRuns(connection, "DROP OPERATOR testschema.## (path, path)")
			defer testutils.AssertQueryRuns(connection, "DROP OPERATOR ### (path, path)")

			expectedOperator := backup.QueryOperator{0, "testschema", "##", "testschema.\"testFunc\"", "path", "path", "testschema.##", "###", "eqsel", "eqjoinsel", true, true}

			results := backup.GetOperators(connection)

			Expect(len(results)).To(Equal(1))
			testutils.ExpectStructsToMatchExcluding(&expectedOperator, &results[0], "Oid")
		})
	})
	Describe("GetOperatorFamilies", func() {
		It("returns a slice of operator families", func() {
			testutils.AssertQueryRuns(connection, "CREATE OPERATOR FAMILY testfam USING hash;")
			defer testutils.AssertQueryRuns(connection, "DROP OPERATOR FAMILY testfam USING hash")

			expectedOperator := backup.QueryOperatorFamily{0, "public", "testfam", "hash"}

			results := backup.GetOperatorFamilies(connection)

			Expect(len(results)).To(Equal(1))
			testutils.ExpectStructsToMatchExcluding(&expectedOperator, &results[0], "Oid")
		})
	})
	Describe("GetOperatorClasses", func() {
		It("returns a slice of operator classes", func() {
			testutils.AssertQueryRuns(connection, "CREATE OPERATOR CLASS testclass FOR TYPE uuid USING hash AS STORAGE uuid")
			defer testutils.AssertQueryRuns(connection, "DROP OPERATOR FAMILY testclass USING hash CASCADE")

			expected := backup.QueryOperatorClass{0, "public", "testclass", "public", "testclass", "hash", "uuid", false, "-", nil, nil}

			results := backup.GetOperatorClasses(connection)

			Expect(len(results)).To(Equal(1))
			testutils.ExpectStructsToMatchExcluding(&expected, &results[0], "Oid")
		})
		It("returns a slice of operator classes with different type and storage type", func() {
			testutils.AssertQueryRuns(connection, "CREATE SCHEMA testschema")
			defer testutils.AssertQueryRuns(connection, "DROP SCHEMA testschema")

			testutils.AssertQueryRuns(connection, "CREATE OPERATOR FAMILY testschema.testfam USING gist;")
			defer testutils.AssertQueryRuns(connection, "DROP OPERATOR FAMILY testschema.testfam USING gist CASCADE")
			testutils.AssertQueryRuns(connection, "CREATE OPERATOR CLASS testclass DEFAULT FOR TYPE uuid USING gist FAMILY testschema.testfam AS STORAGE int")

			expected := backup.QueryOperatorClass{0, "public", "testclass", "testschema", "testfam", "gist", "uuid", true, "integer", nil, nil}

			results := backup.GetOperatorClasses(connection)

			Expect(len(results)).To(Equal(1))
			testutils.ExpectStructsToMatchExcluding(&expected, &results[0], "Oid")
		})
		It("returns a slice of operator classes with operators and functions", func() {
			testutils.AssertQueryRuns(connection, "CREATE OPERATOR CLASS testclass FOR TYPE uuid USING gist AS OPERATOR 1 = RECHECK, OPERATOR 2 < , FUNCTION 1 abs(integer), FUNCTION 2 int4out(integer)")
			defer testutils.AssertQueryRuns(connection, "DROP OPERATOR FAMILY testclass USING gist CASCADE")

			expected := backup.QueryOperatorClass{0, "public", "testclass", "public", "testclass", "gist", "uuid", false, "-", nil, nil}
			expected.Operators = []backup.OperatorClassOperator{{0, 1, "=(uuid,uuid)", true}, {0, 2, "<(uuid,uuid)", false}}
			expected.Functions = []backup.OperatorClassFunction{{0, 1, "abs(integer)"}, {0, 2, "int4out(integer)"}}

			results := backup.GetOperatorClasses(connection)

			Expect(len(results)).To(Equal(1))
			testutils.ExpectStructsToMatchExcluding(&expected, &results[0], "Oid", "Operators.ClassOid", "Functions.ClassOid")
		})
	})
})