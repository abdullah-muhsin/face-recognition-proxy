<?php

namespace App\Services;

use App\Models\AttendanceRecord;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Storage;

class AttendanceRecordPictureStorage
{
    public const CONTENT_TYPE = 'image/jpeg';

    /**
     * @return array{bytes: int, sha256: string}
     */
    public function store(AttendanceRecord $attendanceRecord, Request $request): array
    {
        abort_unless($attendanceRecord->picture_expected, 409, 'Picture upload is not expected for this event.');
        abort_unless($request->headers->get('Content-Type') === self::CONTENT_TYPE, 415, 'Picture content type must be image/jpeg.');

        $input = $request->getContent(true);
        abort_unless(is_resource($input), 500, 'Unable to read picture body.');

        $temp = tmpfile();
        if (! is_resource($temp)) {
            fclose($input);
            abort(500, 'Unable to create temporary picture file.');
        }

        $bytes = 0;
        $head = '';
        $tail = '';
        $hash = hash_init('sha256');
        $maxBytes = $this->maxPictureBytes();

        while (! feof($input)) {
            $chunk = fread($input, 8192);
            if ($chunk === false) {
                fclose($input);
                fclose($temp);
                abort(400, 'Unable to read picture body.');
            }
            if ($chunk === '') {
                continue;
            }

            $bytes += strlen($chunk);
            if ($bytes > $maxBytes) {
                fclose($input);
                fclose($temp);
                abort(413, 'Picture data is too large.');
            }

            if (strlen($head) < 3) {
                $head .= substr($chunk, 0, 3 - strlen($head));
            }
            $tail = substr($tail.$chunk, -2);
            hash_update($hash, $chunk);
            fwrite($temp, $chunk);
        }
        fclose($input);

        abort_if($bytes < 4, 422, 'Picture data is required.');
        abort_unless(
            str_starts_with($head, "\xFF\xD8\xFF") && $tail === "\xFF\xD9",
            422,
            'Picture data is not a complete JPEG image.',
        );

        $pictureHash = hash_final($hash);
        abort_if(
            filled($attendanceRecord->picture_sha256) && $attendanceRecord->picture_sha256 !== $pictureHash,
            409,
            'Picture content changed for existing event.',
        );

        rewind($temp);
        $path = "attendance-record-pictures/{$attendanceRecord->id}.jpg";
        if (! Storage::disk('local')->writeStream($path, $temp)) {
            fclose($temp);
            abort(500, 'Unable to store picture.');
        }
        fclose($temp);

        $attendanceRecord->forceFill([
            'picture_path' => $path,
            'picture_content_type' => self::CONTENT_TYPE,
            'picture_bytes' => $bytes,
            'picture_sha256' => $pictureHash,
        ])->save();

        return [
            'bytes' => $bytes,
            'sha256' => $pictureHash,
        ];
    }

    private function maxPictureBytes(): int
    {
        $maxBytes = config('attendance.picture_max_bytes');
        abort_unless(is_int($maxBytes) && $maxBytes >= 4, 500, 'Picture size limit is misconfigured.');

        return $maxBytes;
    }
}
