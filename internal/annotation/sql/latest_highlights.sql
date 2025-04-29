-- Queries the most recent, non-deleted annotations SINCE a given PK, joining with book asset info.
-- Note: The Go code will substitute the following placeholders before execution:
--   [AEAnnotation] -> Configured annotation attach alias (e.g., from db_objects.annotation_attach_alias)
--   [ZAEANNOTATION] -> Configured annotation table name (e.g., from db_objects.annotation_table)
--   [ZBKLIBRARYASSET] -> Configured library asset table name (e.g., from db_objects.library_asset_table)
SELECT
    A.Z_PK, -- Added Primary Key
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
    A.Z_PK > ? -- Filter by last known PK
  AND
    highlight IS NOT NULL
ORDER BY
    A.Z_PK ASC; -- Order by PK ascending to process in order