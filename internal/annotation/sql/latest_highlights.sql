-- Queries the 10 most recent, non-deleted annotations, joining with book asset info.
-- Note: The Go code will substitute the following placeholders before execution:
--   [AEAnnotation] -> Configured annotation attach alias (e.g., from db_objects.annotation_attach_alias)
--   [ZAEANNOTATION] -> Configured annotation table name (e.g., from db_objects.annotation_table)
--   [ZBKLIBRARYASSET] -> Configured library asset table name (e.g., from db_objects.library_asset_table)
SELECT
    COALESCE(A.ZANNOTATIONSELECTEDTEXT,
             A.ZANNOTATIONREPRESENTATIVETEXT) AS highlight,
    B.ZSORTTITLE                          AS book_title,
    B.ZSORTAUTHOR                         AS book_author
FROM
    [AEAnnotation].[ZAEANNOTATION] A -- Alias and table substituted here
LEFT JOIN
    [ZBKLIBRARYASSET] B ON B.ZASSETID = A.ZANNOTATIONASSETID -- Table substituted here
WHERE
    A.ZANNOTATIONDELETED = 0
  AND
    highlight IS NOT NULL
ORDER BY
    A.ZANNOTATIONCREATIONDATE DESC
LIMIT 10; -- TODO: remove this once ready to query all